package main

import (
	"testing"

	"optimus/internal/config"
)

func TestConfigModelCommand(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	t.Chdir(project)
	for _, args := range [][]string{{"model", "global:7b"}, {"model"}, {"model", "project:3b", "--project"}} {
		if err := runConfig(args); err != nil {
			t.Fatal(err)
		}
	}
	cfg, err := config.Load(project)
	if err != nil || cfg.Models["fast"].Model != "project:3b" {
		t.Fatalf("default not saved: %+v, %v", cfg, err)
	}
	for _, args := range [][]string{nil, {"model", "--bad"}, {"model", "a", "b"}, {"model", "--project"}} {
		if err := runConfig(args); err == nil {
			t.Fatalf("accepted invalid args %v", args)
		}
	}
}

func TestVersionCommand(t *testing.T) {
	for _, arg := range []string{"version", "--version", "-v"} {
		if err := run([]string{arg}); err != nil {
			t.Fatal(err)
		}
	}
}
