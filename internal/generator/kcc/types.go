package kcc

import "fmt"

// Resource represents a generated KCC YAML manifest.
type Resource struct {
	Name    string
	Content []byte
}

// ValidationError indicates a user-supplied feature configuration was invalid.
// The activation handler maps this to HTTP 400 instead of 500.
type ValidationError struct {
	Feature string
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s: %s: %s", e.Feature, e.Field, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Feature, e.Message)
}

func newValidationError(feature, field, msg string) *ValidationError {
	return &ValidationError{Feature: feature, Field: field, Message: msg}
}
