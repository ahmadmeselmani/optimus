// Package model defines the provider-agnostic interface the agent runtime
// uses to talk to language models. Ollama (or any future provider) is an
// adapter behind this interface; the runtime never depends on a provider
// directly.
package model

import "context"

// Role identifies who authored a message in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	// RoleTool carries a tool's result back to the model, in response to a
	// tool call the model itself requested (see internal/agent).
	RoleTool Role = "tool"
)

// Message is one turn in a conversation sent to a model.
type Message struct {
	Role    Role
	Content string
}

// ModelRequest is a provider-agnostic request to generate a completion.
type ModelRequest struct {
	Messages    []Message
	Temperature float64
	MaxTokens   int
}

// ModelResponse is the final, aggregated result of a generation.
type ModelResponse struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
}

// ModelEventType identifies the kind of event emitted while streaming.
type ModelEventType string

const (
	EventToken ModelEventType = "token"
	EventDone  ModelEventType = "done"
	EventError ModelEventType = "error"
)

// ModelEvent is one item streamed back from Model.Stream.
type ModelEvent struct {
	Type     ModelEventType
	Token    string
	Response ModelResponse
	Err      error
}

// Model is the interface the agent runtime depends on. Providers (Ollama,
// future cloud APIs, etc.) implement this interface; the runtime must never
// depend on a provider's concrete type.
type Model interface {
	Generate(ctx context.Context, req ModelRequest) (ModelResponse, error)
	Stream(ctx context.Context, req ModelRequest) (<-chan ModelEvent, error)
	Capabilities() Capabilities
}

// ModelLister is implemented by providers that can enumerate the models
// actually available to them right now (e.g. Ollama's GET /api/tags). Not
// every provider can — a hosted API's catalog isn't "what's installed" the
// way a local Ollama server's is — so callers should type-assert for this
// rather than assume it. Kept out of the core Model interface so adding a
// provider that can't support it doesn't become a breaking change.
type ModelLister interface {
	ListModels(ctx context.Context) ([]InstalledModel, error)
}

// ModelSwitcher is implemented by providers that can change which
// underlying model they talk to without being rebuilt (Ollama just needs a
// different model name per request). Also kept out of the core Model
// interface for the same reason as ModelLister.
type ModelSwitcher interface {
	SwitchModel(name string)
}
