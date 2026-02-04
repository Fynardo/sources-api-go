package superkey

import (
	"errors"
	"strings"
	"testing"
)

func TestForgeError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ForgeError
		expected string
	}{
		{
			name: "with resource",
			err: &ForgeError{
				Step:     "s3",
				Resource: "test-bucket",
				Err:      errors.New("bucket creation failed"),
			},
			expected: `superkey forge failed at step "s3" for resource "test-bucket": bucket creation failed`,
		},
		{
			name: "without resource",
			err: &ForgeError{
				Step: "policy",
				Err:  errors.New("policy creation failed"),
			},
			expected: `superkey forge failed at step "policy": policy creation failed`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestForgeError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	forgeErr := &ForgeError{
		Step: "role",
		Err:  originalErr,
	}

	unwrapped := forgeErr.Unwrap()
	if unwrapped != originalErr {
		t.Errorf("expected original error, got %v", unwrapped)
	}
}

func TestNewForgeError(t *testing.T) {
	forgedApp := &ForgedApplication{
		GUID: "test-guid",
	}
	originalErr := errors.New("test error")

	err := NewForgeError("role", "test-role", originalErr, forgedApp)

	if err.Step != "role" {
		t.Errorf("expected step 'role', got %q", err.Step)
	}

	if err.Resource != "test-role" {
		t.Errorf("expected resource 'test-role', got %q", err.Resource)
	}

	if err.Err != originalErr {
		t.Error("expected original error to be wrapped")
	}

	if err.ForgedApp != forgedApp {
		t.Error("expected forged app to be set")
	}
}

func TestTearDownErrors_Error(t *testing.T) {
	tests := []struct {
		name     string
		errs     *TearDownErrors
		contains []string
	}{
		{
			name:     "no errors",
			errs:     &TearDownErrors{Errors: nil},
			contains: []string{"no teardown errors"},
		},
		{
			name: "single error",
			errs: &TearDownErrors{
				Errors: []error{errors.New("failed to delete bucket")},
			},
			contains: []string{"1 error", "failed to delete bucket"},
		},
		{
			name: "multiple errors",
			errs: &TearDownErrors{
				Errors: []error{
					errors.New("failed to delete bucket"),
					errors.New("failed to delete policy"),
				},
			},
			contains: []string{"2 error", "failed to delete bucket", "failed to delete policy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.errs.Error()
			for _, expected := range tt.contains {
				if !strings.Contains(got, expected) {
					t.Errorf("expected error message to contain %q, got %q", expected, got)
				}
			}
		})
	}
}

func TestTearDownErrors_HasErrors(t *testing.T) {
	noErrors := &TearDownErrors{Errors: nil}
	if noErrors.HasErrors() {
		t.Error("expected HasErrors to return false for nil errors")
	}

	emptyErrors := &TearDownErrors{Errors: []error{}}
	if emptyErrors.HasErrors() {
		t.Error("expected HasErrors to return false for empty errors")
	}

	withErrors := &TearDownErrors{Errors: []error{errors.New("test")}}
	if !withErrors.HasErrors() {
		t.Error("expected HasErrors to return true when errors exist")
	}
}

func TestNewTearDownErrors(t *testing.T) {
	errs := []error{errors.New("error1"), errors.New("error2")}
	tearDownErrors := NewTearDownErrors(errs)

	if len(tearDownErrors.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(tearDownErrors.Errors))
	}
}

func TestAuthenticationError(t *testing.T) {
	originalErr := errors.New("auth not found")
	authErr := &AuthenticationError{Err: originalErr}

	expected := "failed to obtain superkey authentication: auth not found"
	if authErr.Error() != expected {
		t.Errorf("expected %q, got %q", expected, authErr.Error())
	}

	if authErr.Unwrap() != originalErr {
		t.Error("expected original error to be unwrapped")
	}
}

func TestProviderError(t *testing.T) {
	originalErr := errors.New("aws error")
	providerErr := &ProviderError{Provider: "amazon", Err: originalErr}

	expected := `provider "amazon" error: aws error`
	if providerErr.Error() != expected {
		t.Errorf("expected %q, got %q", expected, providerErr.Error())
	}

	if providerErr.Unwrap() != originalErr {
		t.Error("expected original error to be unwrapped")
	}
}
