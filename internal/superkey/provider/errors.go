package provider

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aws/smithy-go"
)

// Error codes for different failure scenarios
const (
	ErrCodeAWSRateLimit            = "SUPERKEY_AWS_RATE_LIMIT"
	ErrCodeInvalidCredentials      = "SUPERKEY_INVALID_CREDENTIALS"
	ErrCodeInsufficientPermissions = "SUPERKEY_INSUFFICIENT_PERMISSIONS"
	ErrCodeResourceConflict        = "SUPERKEY_RESOURCE_CONFLICT"
	ErrCodeAWSServiceError         = "SUPERKEY_AWS_SERVICE_ERROR"
	ErrCodeUnknown                 = "SUPERKEY_UNKNOWN_ERROR"
)

// SuperkeyError represents an error that occurred during superkey operations
type SuperkeyError struct {
	Code      string
	Message   string
	Retryable bool
	Step      string
	AWSError  error
}

// Error implements the error interface
func (e *SuperkeyError) Error() string {
	if e.Step != "" {
		return fmt.Sprintf("[%s] %s (step: %s)", e.Code, e.Message, e.Step)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// ToStatusError returns a user-friendly error message
func (e *SuperkeyError) ToStatusError() string {
	return e.Message
}

// Unwrap returns the underlying AWS error
func (e *SuperkeyError) Unwrap() error {
	return e.AWSError
}

// NewSuperkeyError creates a new SuperkeyError from an AWS error
func NewSuperkeyError(step string, awsErr error) *SuperkeyError {
	if awsErr == nil {
		return nil
	}

	skErr := &SuperkeyError{
		Step:     step,
		AWSError: awsErr,
	}

	var apiErr smithy.APIError
	if errors.As(awsErr, &apiErr) {
		skErr.classifyAWSError(apiErr)
	} else {
		skErr.Code = ErrCodeUnknown
		skErr.Message = awsErr.Error()
		skErr.Retryable = false
	}

	return skErr
}

func (e *SuperkeyError) classifyAWSError(apiErr smithy.APIError) {
	errorCode := apiErr.ErrorCode()
	errorMsg := apiErr.ErrorMessage()

	switch errorCode {
	case "Throttling", "ThrottlingException", "TooManyRequestsException":
		e.Code = ErrCodeAWSRateLimit
		e.Message = "AWS API rate limit exceeded."
		e.Retryable = true
	case "InvalidClientTokenId", "SignatureDoesNotMatch", "AccessDenied":
		e.Code = ErrCodeInvalidCredentials
		e.Message = "Invalid AWS credentials."
		e.Retryable = false
	case "UnauthorizedOperation", "AccessDeniedException":
		e.Code = ErrCodeInsufficientPermissions
		e.Message = fmt.Sprintf("Insufficient permissions: %s", errorMsg)
		e.Retryable = false
	case "EntityAlreadyExists", "BucketAlreadyExists":
		e.Code = ErrCodeResourceConflict
		e.Message = fmt.Sprintf("Resource already exists: %s", errorMsg)
		e.Retryable = true
	case "ServiceUnavailable", "InternalError":
		e.Code = ErrCodeAWSServiceError
		e.Message = "AWS service temporarily unavailable."
		e.Retryable = true
	default:
		if strings.Contains(strings.ToLower(errorMsg), "credential") {
			e.Code = ErrCodeInvalidCredentials
			e.Message = "Invalid AWS credentials."
			e.Retryable = false
		} else {
			e.Code = ErrCodeAWSServiceError
			e.Message = fmt.Sprintf("AWS error: %s", errorMsg)
			e.Retryable = true
		}
	}
}

// WrapError wraps a generic error into a SuperkeyError
func WrapError(step, message string, err error) *SuperkeyError {
	if err == nil {
		return nil
	}

	var skErr *SuperkeyError
	if errors.As(err, &skErr) {
		if skErr.Step == "" {
			skErr.Step = step
		}
		return skErr
	}

	skErr = NewSuperkeyError(step, err)
	if skErr != nil {
		return skErr
	}

	return &SuperkeyError{
		Code:      ErrCodeUnknown,
		Message:   message,
		Retryable: false,
		Step:      step,
		AWSError:  err,
	}
}

// IsRetryable returns whether the error can be retried
func IsRetryable(err error) bool {
	var skErr *SuperkeyError
	if errors.As(err, &skErr) {
		return skErr.Retryable
	}
	return false
}
