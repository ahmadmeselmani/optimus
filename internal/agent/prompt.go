package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// systemPrompt describes the tool-call protocol and the available tools.
// The model must reply with exactly one JSON object per turn — see action
// in agent.go for why a fixed protocol is used instead of a provider's
// native function-calling.
func (a *Agent) systemPrompt() string {
	var b strings.Builder

	b.WriteString("You are Optimus, a local coding assistant. You can inspect the user's " +
		"repository using the tools listed below, but you cannot modify anything yet.\n\n")
	b.WriteString("Respond with exactly one JSON object and nothing else: no prose outside " +
		"it, no markdown code fences.\n\n")
	b.WriteString("To call a tool:\n")
	b.WriteString(`{"type":"tool_call","tool":"<tool name>","input":{...}}` + "\n\n")
	b.WriteString("Once you have enough information to answer the user, respond with:\n")
	b.WriteString(`{"type":"final_answer","content":"<your answer>"}` + "\n\n")
	b.WriteString("There is no user available to answer follow-up questions. If you need to know " +
		"something about the repository — its layout, whether a file exists, what a file " +
		"contains — call a tool to find out yourself; never ask the user or stop to describe " +
		"what you're about to do in plain text.\n\n")
	b.WriteString("Available tools:\n")

	for _, t := range a.Tools.List() {
		schema, _ := json.Marshal(t.Schema())
		fmt.Fprintf(&b, "- %s: %s\n  input schema: %s\n", t.Name(), t.Description(), schema)
	}

	if a.RepoOverview != "" {
		fmt.Fprintf(&b, "\n%s\n", a.RepoOverview)
	}

	fmt.Fprintf(&b, "\nYou have at most %d tool calls before you must give a final answer. "+
		"Use them efficiently — prefer search_code or list_directory to find the right file "+
		"before read_file-ing it in full.\n", a.maxToolCalls())

	return b.String()
}
