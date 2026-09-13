package model

// Capabilities describes what a Model implementation supports, so the
// runtime can adapt (e.g. fall back to non-streaming generation) without
// depending on the concrete provider.
type Capabilities struct {
	Streaming        bool
	MaxContextTokens int
}
