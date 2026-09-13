package model

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaListModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Write([]byte(`{"models":[
			{"name":"qwen2.5:3b","details":{"parameter_size":"3.1B","quantization_level":"Q4_K_M"}},
			{"name":"gemma4:e2b","details":{"parameter_size":"5.1B","quantization_level":"Q4_K_M"}}
		]}`))
	}))
	defer srv.Close()

	o := NewOllama(OllamaConfig{BaseURL: srv.URL})
	models, err := o.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2: %+v", len(models), models)
	}
	if models[0].Name != "qwen2.5:3b" || models[0].ParameterSize != "3.1B" || models[0].Quantization != "Q4_K_M" {
		t.Fatalf("unexpected first model: %+v", models[0])
	}
}

func TestOllamaListModelsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer srv.Close()

	o := NewOllama(OllamaConfig{BaseURL: srv.URL})
	if _, err := o.ListModels(context.Background()); err == nil {
		t.Fatal("expected error on non-200 response")
	}
}

func TestOllamaSwitchModel(t *testing.T) {
	o := NewOllama(OllamaConfig{Model: "qwen2.5:3b"})
	if got := o.ModelName(); got != "qwen2.5:3b" {
		t.Fatalf("got %q, want qwen2.5:3b", got)
	}

	o.SwitchModel("qwen2.5-coder:3b")
	if got := o.ModelName(); got != "qwen2.5-coder:3b" {
		t.Fatalf("got %q, want qwen2.5-coder:3b", got)
	}
}

func TestOllamaImplementsOptionalInterfaces(t *testing.T) {
	o := NewOllama(OllamaConfig{})
	var _ ModelLister = o
	var _ ModelSwitcher = o
}
