package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultModelPersistence(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".optimus")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	initial := "# My settings\nmodels:\n  fast:\n    base_url: http://custom:11434\n  reasoning:\n    model: reasoning-model\npermissions:\n  shell: deny\ncustom_setting: retained\n"
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveDefaultModel(project, "coder:7b", false); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(project)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Models["fast"].Model != "coder:7b" || cfg.Models["fast"].BaseURL != "http://custom:11434" || cfg.Permissions.Shell != "deny" || cfg.Models["reasoning"].Model != "reasoning-model" {
		t.Fatalf("settings not preserved: %+v", cfg)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "# My settings") || !strings.Contains(string(data), "custom_setting: retained") {
		t.Fatalf("lost settings: %s", data)
	}
	if _, err := SaveDefaultModel(project, "project-model", true); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveDefaultModel(project, "another-global", false); err != nil {
		t.Fatal(err)
	}
	cfg, err = Load(project)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Models["fast"].Model != "project-model" || cfg.Models["fast"].Provider != "ollama" || cfg.Models["fast"].BaseURL != "http://custom:11434" {
		t.Fatalf("incorrect project inheritance: %+v", cfg.Models["fast"])
	}
}

func TestDefaultModelInvalidConfigUnchanged(t *testing.T) {
	for _, input := range []string{"models: [bad]\n", "models:\n  fast: invalid\n", "[invalid\n", "context:\n  max_tokens: invalid\n", "models:\n  fast:\n    model: one\n    model: two\n"} {
		project := t.TempDir()
		dir := filepath.Join(project, ".optimus")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(path, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := SaveDefaultModel(project, "coder", true); err == nil {
			t.Fatal("expected invalid config error")
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != input {
			t.Fatalf("changed config: %s, %v", data, err)
		}
	}
	for _, name := range []string{"", "two models", "model\n"} {
		if _, err := SaveDefaultModel(t.TempDir(), name, true); err == nil {
			t.Fatalf("accepted invalid name %q", name)
		}
	}
}
