package tools

import "sort"

// Registry holds the tools available to a given agent run. The runtime
// decides which tools to register — it should offer only tools appropriate
// to the current task (PLAN.md §15), not a fixed global set.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry builds an empty Registry.
func NewRegistry() *Registry {
	return &Registry{tools: map[string]Tool{}}
}

// Register adds a tool, replacing any existing tool with the same name.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get looks up a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

// List returns the registered tools sorted by name, for stable prompt
// rendering.
func (r *Registry) List() []Tool {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]Tool, len(names))
	for i, name := range names {
		out[i] = r.tools[name]
	}
	return out
}
