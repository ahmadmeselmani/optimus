package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"optimus/internal/config"
	"optimus/internal/model"
)

// Embed the model interface: only switching is exercised by these commands.
type switchableModel struct {
	model.Model
	name string
}

func (m *switchableModel) SwitchModel(name string) { m.name = name }

func TestModelDefaultCommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	provider := &switchableModel{}
	m := &Model{cfg: config.Default(), cwd: t.TempDir(), model: provider}
	output, _ := cmdModel(m, []string{"default", "coder:7b", "--project"})
	if !strings.Contains(output, "Saved default") || provider.name != "coder:7b" {
		t.Fatal(output)
	}
	cfg, err := config.Load(m.cwd)
	if err != nil || cfg.Models["fast"].Model != "coder:7b" {
		t.Fatalf("not persisted: %+v %v", cfg, err)
	}
	output, _ = cmdModel(m, []string{"session-only"})
	if provider.name != "session-only" {
		t.Fatal(output)
	}
	cfg, err = config.Load(m.cwd)
	if err != nil || cfg.Models["fast"].Model != "coder:7b" {
		t.Fatal("session switch changed default")
	}
	output, _ = cmdModel(m, []string{"default", "--project"})
	if !strings.Contains(output, "Saved default") {
		t.Fatal(output)
	}
	cfg, err = config.Load(m.cwd)
	if err != nil || cfg.Models["fast"].Model != "session-only" {
		t.Fatal("did not save active model")
	}
	path := filepath.Join(m.cwd, ".optimus", "config.yaml")
	if err := os.WriteFile(path, []byte("models: [invalid]"), 0600); err != nil {
		t.Fatal(err)
	}
	output, _ = cmdModel(m, []string{"default", "other", "--project"})
	if !strings.Contains(output, "Could not save") || provider.name != "session-only" {
		t.Fatalf("switched despite failed save: %s", output)
	}
}
