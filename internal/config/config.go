// Package config loads Optimus configuration from global and project-level
// config.yaml files, with the project config overriding the global one.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ModelConfig describes a single named model class (e.g. "fast", "reasoning").
type ModelConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"`
}

// ContextConfig bounds how much context the agent may build for a task.
type ContextConfig struct {
	MaxTokens int `yaml:"max_tokens"`
}

// AgentConfig controls agent-loop limits.
type AgentConfig struct {
	MaxAttempts  int `yaml:"max_attempts"`
	MaxToolCalls int `yaml:"max_tool_calls"`
}

// PermissionsConfig controls what the runtime is allowed to do without asking.
type PermissionsConfig struct {
	Read     string `yaml:"read"`
	Write    string `yaml:"write"`
	Shell    string `yaml:"shell"`
	GitWrite string `yaml:"git_write"`
	Network  string `yaml:"network"`
}

// LoggingConfig controls log verbosity.
type LoggingConfig struct {
	Level string `yaml:"level"`
}

// Config is the fully-resolved Optimus configuration.
type Config struct {
	Models      map[string]ModelConfig `yaml:"models"`
	Context     ContextConfig          `yaml:"context"`
	Agent       AgentConfig            `yaml:"agent"`
	Permissions PermissionsConfig      `yaml:"permissions"`
	Logging     LoggingConfig          `yaml:"logging"`
}

// Default returns the built-in configuration used when no config files are
// present. It targets a local Ollama instance running qwen2.5:3b.
func Default() Config {
	return Config{
		Models: map[string]ModelConfig{
			"fast": {
				Provider: "ollama",
				Model:    "qwen2.5:3b",
				BaseURL:  "http://localhost:11434",
			},
			"reasoning": {
				Provider: "ollama",
				Model:    "qwen2.5:3b",
				BaseURL:  "http://localhost:11434",
			},
		},
		Context: ContextConfig{
			MaxTokens: 16000,
		},
		Agent: AgentConfig{
			MaxAttempts:  3,
			MaxToolCalls: 15,
		},
		Permissions: PermissionsConfig{
			Read:     "allow",
			Write:    "allow",
			Shell:    "ask",
			GitWrite: "ask",
			Network:  "deny",
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

// Load resolves configuration by starting from Default(), then merging
// ~/.optimus/config.yaml, then projectDir/.optimus/config.yaml on top. Missing
// files are not an error.
func Load(projectDir string) (Config, error) {
	cfg := Default()

	if home, err := os.UserHomeDir(); err == nil {
		if err := mergeFile(&cfg, filepath.Join(home, ".optimus", "config.yaml")); err != nil {
			return cfg, err
		}
	}

	if err := mergeFile(&cfg, filepath.Join(projectDir, ".optimus", "config.yaml")); err != nil {
		return cfg, err
	}

	return cfg, nil
}

func mergeFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var override Config
	if err := yaml.Unmarshal(data, &override); err != nil {
		return err
	}

	merge(cfg, override)
	return nil
}

func merge(base *Config, override Config) {
	for name, m := range override.Models {
		if base.Models == nil {
			base.Models = map[string]ModelConfig{}
		}
		current := base.Models[name]
		if m.Provider != "" {
			current.Provider = m.Provider
		}
		if m.Model != "" {
			current.Model = m.Model
		}
		if m.BaseURL != "" {
			current.BaseURL = m.BaseURL
		}
		base.Models[name] = current
	}
	if override.Context.MaxTokens != 0 {
		base.Context.MaxTokens = override.Context.MaxTokens
	}
	if override.Agent.MaxAttempts != 0 {
		base.Agent.MaxAttempts = override.Agent.MaxAttempts
	}
	if override.Agent.MaxToolCalls != 0 {
		base.Agent.MaxToolCalls = override.Agent.MaxToolCalls
	}
	if override.Permissions.Read != "" {
		base.Permissions.Read = override.Permissions.Read
	}
	if override.Permissions.Write != "" {
		base.Permissions.Write = override.Permissions.Write
	}
	if override.Permissions.Shell != "" {
		base.Permissions.Shell = override.Permissions.Shell
	}
	if override.Permissions.GitWrite != "" {
		base.Permissions.GitWrite = override.Permissions.GitWrite
	}
	if override.Permissions.Network != "" {
		base.Permissions.Network = override.Permissions.Network
	}
	if override.Logging.Level != "" {
		base.Logging.Level = override.Logging.Level
	}
}

// SaveDefaultModel updates only models.fast.model, preserving other settings.
// Project defaults override global defaults, following Load's precedence.
func SaveDefaultModel(projectDir, name string, project bool) (string, error) {
	if name == "" || strings.ContainsAny(name, " \t\r\n") {
		return "", fmt.Errorf("model name must be non-empty and contain no whitespace")
	}
	dir := projectDir
	if !project {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = home
	}
	path := filepath.Join(dir, ".optimus", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}
	var existing Config
	if err := yaml.Unmarshal(data, &existing); err != nil {
		return "", fmt.Errorf("read config: %w", err)
	}
	if len(doc.Content) == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	node := doc.Content[0]
	for _, key := range []string{"models", "fast", "model"} {
		if node.Kind != yaml.MappingNode {
			return "", fmt.Errorf("config entry containing %q must be a mapping", key)
		}
		var child *yaml.Node
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == key {
				child = node.Content[i+1]
				break
			}
		}
		if child == nil {
			child = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			node.Content = append(node.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, child)
		}
		node = child
	}
	node.Kind, node.Tag, node.Value, node.Content = yaml.ScalarNode, "!!str", name, nil
	data, err = yaml.Marshal(&doc)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}
