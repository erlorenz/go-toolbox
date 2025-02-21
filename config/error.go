package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a configuration validation error
type ValidationError struct {
	Field  string `json:"field"`
	Value  any    `json:"value"`
	Reason string `json:"reason"`
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s': %s", e.Field, e.Reason)
}

// MultiError holds multiple errors that occurred during parsing
type MultiError struct {
	Errors []error `json:"errors"`
}

func (m *MultiError) Error() string {
	if len(m.Errors) == 0 {
		return "no errors"
	}

	errMsgs := make([]string, len(m.Errors))
	for i, err := range m.Errors {
		errMsgs[i] = err.Error()
	}

	return fmt.Sprintf("%d error(s) occurred:\n- %s",
		len(m.Errors), strings.Join(errMsgs, "\n- "))
}
