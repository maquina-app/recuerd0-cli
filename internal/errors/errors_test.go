package errors

import "testing"

func TestCLIError_Error(t *testing.T) {
	err := NewError("something went wrong")
	if err.Error() != "something went wrong" {
		t.Errorf("expected 'something went wrong', got '%s'", err.Error())
	}
}

func TestNewErrorf(t *testing.T) {
	err := NewErrorf("failed to %s: %d", "connect", 500)
	if err.Message != "failed to connect: 500" {
		t.Errorf("unexpected message: %s", err.Message)
	}
	if err.ExitCode != ExitError {
		t.Errorf("expected exit code %d, got %d", ExitError, err.ExitCode)
	}
}

func TestTypedConstructors(t *testing.T) {
	tests := []struct {
		name     string
		err      *CLIError
		code     string
		exitCode int
		status   int
	}{
		{"InvalidArgs", NewInvalidArgsError("bad args"), CodeInvalidArgs, ExitInvalidArgs, 0},
		{"Auth", NewAuthError("no token"), CodeAuth, ExitAuthFailure, 401},
		{"Forbidden", NewForbiddenError("no access"), CodeForbidden, ExitForbidden, 403},
		{"NotFound", NewNotFoundError("not found"), CodeNotFound, ExitNotFound, 404},
		{"Validation", NewValidationError("invalid"), CodeValidation, ExitValidation, 422},
		{"Network", NewNetworkError("timeout"), CodeNetwork, ExitNetwork, 0},
		{"RateLimited", NewRateLimitedError("slow down"), CodeRateLimited, ExitRateLimited, 429},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Code != tt.code {
				t.Errorf("expected code %s, got %s", tt.code, tt.err.Code)
			}
			if tt.err.ExitCode != tt.exitCode {
				t.Errorf("expected exit code %d, got %d", tt.exitCode, tt.err.ExitCode)
			}
			if tt.status != 0 && tt.err.Status != tt.status {
				t.Errorf("expected status %d, got %d", tt.status, tt.err.Status)
			}
		})
	}
}

func TestFromHTTPStatus(t *testing.T) {
	tests := []struct {
		status   int
		code     string
		exitCode int
	}{
		{401, CodeAuth, ExitAuthFailure},
		{403, CodeForbidden, ExitForbidden},
		{404, CodeNotFound, ExitNotFound},
		{422, CodeValidation, ExitValidation},
		{429, CodeRateLimited, ExitRateLimited},
		{400, CodeError, ExitError},
		{500, CodeError, ExitError},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.status)), func(t *testing.T) {
			err := FromHTTPStatus(tt.status, "test")
			if err.Code != tt.code {
				t.Errorf("status %d: expected code %s, got %s", tt.status, tt.code, err.Code)
			}
			if err.ExitCode != tt.exitCode {
				t.Errorf("status %d: expected exit code %d, got %d", tt.status, tt.exitCode, err.ExitCode)
			}
		})
	}
}
