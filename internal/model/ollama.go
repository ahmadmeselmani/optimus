package model

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaConfig configures the Ollama adapter.
type OllamaConfig struct {
	BaseURL string
	Model   string
	Timeout time.Duration
}

// Ollama is a Model implementation backed by a local Ollama server's
// /api/chat endpoint. It is the only piece of Optimus allowed to know about
// Ollama's wire format.
type Ollama struct {
	cfg    OllamaConfig
	client *http.Client
}

// NewOllama builds an Ollama-backed Model from cfg, filling in defaults for
// any unset fields.
func NewOllama(cfg OllamaConfig) *Ollama {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 2 * time.Minute
	}
	return &Ollama{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

func (o *Ollama) Capabilities() Capabilities {
	return Capabilities{
		Streaming:        true,
		MaxContextTokens: 8192,
	}
}

// ModelName returns the model this adapter currently sends requests to.
func (o *Ollama) ModelName() string {
	return o.cfg.Model
}

// SwitchModel changes which model this adapter sends requests to. It takes
// effect on the next Generate/Stream call; an in-flight generation already
// has its request built and is unaffected. Implements ModelSwitcher.
func (o *Ollama) SwitchModel(name string) {
	o.cfg.Model = name
}

// InstalledModel describes one model Ollama reports as locally available.
type InstalledModel struct {
	Name          string
	ParameterSize string
	Quantization  string
}

type ollamaTagsResponse struct {
	Models []struct {
		Name    string `json:"name"`
		Details struct {
			ParameterSize string `json:"parameter_size"`
			Quantization  string `json:"quantization_level"`
		} `json:"details"`
	} `json:"models"`
}

// ListModels queries Ollama's /api/tags for the models actually pulled
// locally — distinct from Capabilities, which describes the one model this
// adapter is currently configured to use. Implements ModelLister.
func (o *Ollama) ListModels(ctx context.Context) ([]InstalledModel, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, o.cfg.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("model: build request: %w", err)
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return nil, fmt.Errorf("model: ollama returned status %d: %s", resp.StatusCode, buf.String())
	}

	var body ollamaTagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("model: decode /api/tags response: %w", err)
	}

	out := make([]InstalledModel, len(body.Models))
	for i, m := range body.Models {
		out[i] = InstalledModel{
			Name:          m.Name,
			ParameterSize: m.Details.ParameterSize,
			Quantization:  m.Details.Quantization,
		}
	}
	return out, nil
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Options  *ollamaChatOptions  `json:"options,omitempty"`
}

type ollamaChatOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaChatChunk struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Done            bool   `json:"done"`
	PromptEvalCount int    `json:"prompt_eval_count"`
	EvalCount       int    `json:"eval_count"`
	Error           string `json:"error"`
}

func toOllamaMessages(msgs []Message) []ollamaChatMessage {
	out := make([]ollamaChatMessage, len(msgs))
	for i, m := range msgs {
		out[i] = ollamaChatMessage{Role: string(m.Role), Content: m.Content}
	}
	return out
}

// Stream sends req to Ollama's chat endpoint and emits token-by-token
// events as the server streams its NDJSON response, followed by a final
// EventDone (or EventError on failure).
func (o *Ollama) Stream(ctx context.Context, req ModelRequest) (<-chan ModelEvent, error) {
	body := ollamaChatRequest{
		Model:    o.cfg.Model,
		Messages: toOllamaMessages(req.Messages),
		Stream:   true,
	}
	if req.Temperature != 0 || req.MaxTokens != 0 {
		body.Options = &ollamaChatOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("model: encode request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.cfg.BaseURL+"/api/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("model: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		var buf bytes.Buffer
		buf.ReadFrom(resp.Body)
		return nil, fmt.Errorf("model: ollama returned status %d: %s", resp.StatusCode, buf.String())
	}

	events := make(chan ModelEvent)
	go func() {
		defer resp.Body.Close()
		defer close(events)
		send := func(event ModelEvent) bool {
			select {
			case events <- event:
				return true
			case <-ctx.Done():
				return false
			}
		}

		var content bytes.Buffer
		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		for scanner.Scan() {
			line := scanner.Bytes()
			if len(bytes.TrimSpace(line)) == 0 {
				continue
			}

			var chunk ollamaChatChunk
			if err := json.Unmarshal(line, &chunk); err != nil {
				send(ModelEvent{Type: EventError, Err: fmt.Errorf("model: decode chunk: %w", err)})
				return
			}
			if chunk.Error != "" {
				send(ModelEvent{Type: EventError, Err: fmt.Errorf("model: ollama error: %s", chunk.Error)})
				return
			}

			if chunk.Message.Content != "" {
				content.WriteString(chunk.Message.Content)
				if !send(ModelEvent{Type: EventToken, Token: chunk.Message.Content}) {
					return
				}
			}

			if chunk.Done {
				send(ModelEvent{
					Type: EventDone,
					Response: ModelResponse{
						Content:          content.String(),
						PromptTokens:     chunk.PromptEvalCount,
						CompletionTokens: chunk.EvalCount,
					},
				})
				return
			}
		}

		if err := scanner.Err(); err != nil {
			send(ModelEvent{Type: EventError, Err: fmt.Errorf("model: read stream: %w", err)})
		}
	}()

	return events, nil
}

// Generate runs Stream to completion and returns the aggregated response.
func (o *Ollama) Generate(ctx context.Context, req ModelRequest) (ModelResponse, error) {
	events, err := o.Stream(ctx, req)
	if err != nil {
		return ModelResponse{}, err
	}

	for ev := range events {
		switch ev.Type {
		case EventError:
			return ModelResponse{}, ev.Err
		case EventDone:
			if ev.Response.Content == "" {
				return ev.Response, ErrEmptyResponse
			}
			return ev.Response, nil
		}
	}

	return ModelResponse{}, ErrEmptyResponse
}
