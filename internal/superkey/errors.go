package superkey

import (
	"fmt"
	"strings"
)

// ForgeError wraps errors during resource creation with step context
type ForgeError struct {
	Step      string            // Which step failed (s3, policy, role, etc.)
	Resource  string            // Resource name that failed
	Err       error             // Underlying error
	ForgedApp *ForgedApplication // Partial state for rollback
}

func (e *ForgeError) Error() string {
	if e.Resource != "" {
		return fmt.Sprintf("superkey forge failed at step %q for resource %q: %v", e.Step, e.Resource, e.Err)
	}
	return fmt.Sprintf("superkey forge failed at step %q: %v", e.Step, e.Err)
}

func (e *ForgeError) Unwrap() error {
	return e.Err
}

// NewForgeError creates a new ForgeError
func NewForgeError(step, resource string, err error, forgedApp *ForgedApplication) *ForgeError {
	return &ForgeError{
		Step:      step,
		Resource:  resource,
		Err:       err,
		ForgedApp: forgedApp,
	}
}

// TearDownErrors collects all errors during cleanup
type TearDownErrors struct {
	Errors []error
}

func (e *TearDownErrors) Error() string {
	if len(e.Errors) == 0 {
		return "no teardown errors"
	}

	var msgs []string
	for _, err := range e.Errors {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("teardown encountered %d error(s): %s", len(e.Errors), strings.Join(msgs, "; "))
}

func (e *TearDownErrors) HasErrors() bool {
	return len(e.Errors) > 0
}

// NewTearDownErrors creates a new TearDownErrors from a slice of errors
func NewTearDownErrors(errs []error) *TearDownErrors {
	return &TearDownErrors{Errors: errs}
}

// AuthenticationError indicates a failure to obtain superkey authentication
type AuthenticationError struct {
	Err error
}

func (e *AuthenticationError) Error() string {
	return fmt.Sprintf("failed to obtain superkey authentication: %v", e.Err)
}

func (e *AuthenticationError) Unwrap() error {
	return e.Err
}

// ProviderError indicates a failure with the cloud provider
type ProviderError struct {
	Provider string
	Err      error
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("provider %q error: %v", e.Provider, e.Err)
}

func (e *ProviderError) Unwrap() error {
	return e.Err
}
