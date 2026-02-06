package errors

import "fmt"

// Exit codes
const (
	ExitSuccess     = 0
	ExitError       = 1
	ExitInvalidArgs = 2
	ExitAuthFailure = 3
	ExitForbidden   = 4
	ExitNotFound    = 5
	ExitValidation  = 6
	ExitNetwork     = 7
	ExitRateLimited = 8
)

// Error codes
const (
	CodeError       = "ERROR"
	CodeInvalidArgs = "INVALID_ARGS"
	CodeAuth        = "AUTH_ERROR"
	CodeForbidden   = "FORBIDDEN"
	CodeNotFound    = "NOT_FOUND"
	CodeValidation  = "VALIDATION_ERROR"
	CodeNetwork     = "NETWORK_ERROR"
	CodeRateLimited = "RATE_LIMITED"
)

// CLIError represents a typed CLI error with exit code and HTTP status.
type CLIError struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Status   int    `json:"status,omitempty"`
	ExitCode int    `json:"-"`
}

func (e *CLIError) Error() string {
	return e.Message
}

func NewError(message string) *CLIError {
	return &CLIError{Code: CodeError, Message: message, ExitCode: ExitError}
}

func NewErrorf(format string, args ...interface{}) *CLIError {
	return NewError(fmt.Sprintf(format, args...))
}

func NewInvalidArgsError(message string) *CLIError {
	return &CLIError{Code: CodeInvalidArgs, Message: message, ExitCode: ExitInvalidArgs}
}

func NewAuthError(message string) *CLIError {
	return &CLIError{Code: CodeAuth, Message: message, Status: 401, ExitCode: ExitAuthFailure}
}

func NewForbiddenError(message string) *CLIError {
	return &CLIError{Code: CodeForbidden, Message: message, Status: 403, ExitCode: ExitForbidden}
}

func NewNotFoundError(message string) *CLIError {
	return &CLIError{Code: CodeNotFound, Message: message, Status: 404, ExitCode: ExitNotFound}
}

func NewValidationError(message string) *CLIError {
	return &CLIError{Code: CodeValidation, Message: message, Status: 422, ExitCode: ExitValidation}
}

func NewNetworkError(message string) *CLIError {
	return &CLIError{Code: CodeNetwork, Message: message, ExitCode: ExitNetwork}
}

func NewRateLimitedError(message string) *CLIError {
	return &CLIError{Code: CodeRateLimited, Message: message, Status: 429, ExitCode: ExitRateLimited}
}

// FromHTTPStatus maps an HTTP status code to a typed CLIError.
func FromHTTPStatus(status int, message string) *CLIError {
	switch {
	case status == 401:
		return NewAuthError(message)
	case status == 403:
		return NewForbiddenError(message)
	case status == 404:
		return NewNotFoundError(message)
	case status == 422:
		return NewValidationError(message)
	case status == 429:
		return NewRateLimitedError(message)
	case status >= 400 && status < 500:
		return &CLIError{Code: CodeError, Message: message, Status: status, ExitCode: ExitError}
	case status >= 500:
		return &CLIError{Code: CodeError, Message: fmt.Sprintf("Server error (%d): %s", status, message), Status: status, ExitCode: ExitError}
	default:
		return NewError(message)
	}
}
