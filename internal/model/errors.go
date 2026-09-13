package model

import "errors"

var (
	// ErrEmptyResponse is returned when a provider returns no content.
	ErrEmptyResponse = errors.New("model: empty response")
	// ErrUnavailable is returned when a provider cannot be reached.
	ErrUnavailable = errors.New("model: provider unavailable")
)
