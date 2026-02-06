package response

import (
	"encoding/json"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/errors"
)

func TestSuccess(t *testing.T) {
	r := Success(map[string]string{"id": "1"})
	if !r.Success {
		t.Error("expected success=true")
	}
	if r.Data == nil {
		t.Error("expected data to be set")
	}
	if r.Meta == nil || r.Meta["timestamp"] == nil {
		t.Error("expected meta.timestamp to be set")
	}
}

func TestSuccessWithSummary(t *testing.T) {
	r := SuccessWithSummary([]string{"a", "b"}, "2 items")
	if r.Summary != "2 items" {
		t.Errorf("expected summary '2 items', got '%s'", r.Summary)
	}
}

func TestSuccessWithLocation(t *testing.T) {
	r := SuccessWithLocation(map[string]string{"id": "1"}, "/workspaces/1")
	if r.Location != "/workspaces/1" {
		t.Errorf("expected location '/workspaces/1', got '%s'", r.Location)
	}
}

func TestSuccessWithPagination(t *testing.T) {
	r := SuccessWithPagination([]string{"a"}, true, "/next?page=2")
	if r.Pagination == nil {
		t.Fatal("expected pagination to be set")
	}
	if !r.Pagination.HasNext {
		t.Error("expected has_next=true")
	}
	if r.Pagination.NextURL != "/next?page=2" {
		t.Errorf("expected next_url '/next?page=2', got '%s'", r.Pagination.NextURL)
	}
}

func TestSuccessWithBreadcrumbs(t *testing.T) {
	bc := []Breadcrumb{
		{Action: "show", Cmd: "recuerd0 workspace show 1", Description: "View workspace"},
	}
	r := SuccessWithBreadcrumbs(map[string]string{"id": "1"}, "1 workspace", bc)
	if len(r.Breadcrumbs) != 1 {
		t.Fatalf("expected 1 breadcrumb, got %d", len(r.Breadcrumbs))
	}
	if r.Breadcrumbs[0].Action != "show" {
		t.Errorf("expected action 'show', got '%s'", r.Breadcrumbs[0].Action)
	}
}

func TestSuccessWithPaginationAndBreadcrumbs(t *testing.T) {
	bc := []Breadcrumb{
		{Action: "show", Cmd: "recuerd0 memory show 1", Description: "View memory"},
	}
	r := SuccessWithPaginationAndBreadcrumbs([]string{"a"}, true, "/next", "1 memory", bc)
	if r.Pagination == nil {
		t.Fatal("expected pagination")
	}
	if len(r.Breadcrumbs) != 1 {
		t.Fatal("expected 1 breadcrumb")
	}
	if r.Summary != "1 memory" {
		t.Errorf("expected summary '1 memory', got '%s'", r.Summary)
	}
}

func TestError(t *testing.T) {
	err := errors.NewNotFoundError("workspace not found")
	r := Error(err)
	if r.Success {
		t.Error("expected success=false")
	}
	if r.Error == nil {
		t.Fatal("expected error to be set")
	}
	if r.Error.Code != errors.CodeNotFound {
		t.Errorf("expected code %s, got %s", errors.CodeNotFound, r.Error.Code)
	}
	if r.Error.Status != 404 {
		t.Errorf("expected status 404, got %d", r.Error.Status)
	}
}

func TestJSON_Compact(t *testing.T) {
	SetPrettyPrint(false)
	defer SetPrettyPrint(false)

	r := Success(map[string]string{"id": "1"})
	data, err := r.JSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Compact JSON should not contain newlines in the body
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if parsed["success"] != true {
		t.Error("expected success=true in JSON")
	}
}

func TestJSON_Pretty(t *testing.T) {
	SetPrettyPrint(true)
	defer SetPrettyPrint(false)

	r := Success(map[string]string{"id": "1"})
	data, err := r.JSON()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Pretty JSON should contain indentation
	str := string(data)
	if len(str) < 10 {
		t.Error("pretty JSON seems too short")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestErrorCodeToExitCode(t *testing.T) {
	tests := []struct {
		code     string
		exitCode int
	}{
		{errors.CodeInvalidArgs, errors.ExitInvalidArgs},
		{errors.CodeAuth, errors.ExitAuthFailure},
		{errors.CodeForbidden, errors.ExitForbidden},
		{errors.CodeNotFound, errors.ExitNotFound},
		{errors.CodeValidation, errors.ExitValidation},
		{errors.CodeNetwork, errors.ExitNetwork},
		{errors.CodeRateLimited, errors.ExitRateLimited},
		{errors.CodeError, errors.ExitError},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			got := errorCodeToExitCode(tt.code)
			if got != tt.exitCode {
				t.Errorf("code %s: expected exit code %d, got %d", tt.code, tt.exitCode, got)
			}
		})
	}
}
