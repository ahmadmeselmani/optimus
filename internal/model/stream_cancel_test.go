package model

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type streamTestTransport func(*http.Request) (*http.Response, error)

func (f streamTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type observedStreamBody struct {
	io.Reader
	read   chan struct{}
	closed chan struct{}
}

func (b *observedStreamBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	select {
	case b.read <- struct{}{}:
	default:
	}
	return n, err
}

func (b *observedStreamBody) Close() error { close(b.closed); return nil }

func TestStreamCancellationWithoutConsumer(t *testing.T) {
	for _, chunk := range []string{`{"message":{"content":"Hello"}}`, `{"done":true}`, `{"error":"failed"}`, `invalid JSON`} {
		t.Run(chunk, func(t *testing.T) {
			body := &observedStreamBody{Reader: strings.NewReader(chunk + "\n"), read: make(chan struct{}, 1), closed: make(chan struct{})}
			o := NewOllama(OllamaConfig{Model: "test"})
			o.client.Transport = streamTestTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
			})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if _, err := o.Stream(ctx, ModelRequest{}); err != nil {
				t.Fatal(err)
			}
			select {
			case <-body.read:
			case <-time.After(time.Second):
				t.Fatal("stream did not read response")
			}
			// The UI abandons the channel when stopping a reply.
			cancel()
			select {
			case <-body.closed:
			case <-time.After(time.Second):
				t.Fatal("canceled stream kept response body open")
			}
		})
	}
}
