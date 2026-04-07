package commands

import (
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
)

func TestWorkspaceContextDefaults(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data: map[string]interface{}{
			"workspace":       map[string]interface{}{"id": "5", "name": "Rails Patterns"},
			"pinned_memories": []interface{}{},
			"stats":           map[string]interface{}{"total_memories": 42, "total_pinned": 0, "returned_pinned": 0},
		},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	// Reset flags to defaults to isolate from other tests.
	workspaceContextLimit = 10
	workspaceContextNoBody = false
	workspaceContextMaxBodyChars = 500
	defer func() {
		workspaceContextLimit = 10
		workspaceContextNoBody = false
		workspaceContextMaxBodyChars = 500
	}()

	RunTestCommand(func() {
		workspaceContextCmd.Run(workspaceContextCmd, []string{"5"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if len(mock.GetCalls) != 1 {
		t.Fatalf("expected 1 Get call, got %d", len(mock.GetCalls))
	}
	path := mock.GetCalls[0].Path
	if !strings.HasPrefix(path, "/workspaces/5/context?") {
		t.Errorf("unexpected path prefix: %s", path)
	}
	if !strings.Contains(path, "limit=10") {
		t.Errorf("expected limit=10 in path, got: %s", path)
	}
	if !strings.Contains(path, "max_body_chars=500") {
		t.Errorf("expected max_body_chars=500 in path, got: %s", path)
	}
	if strings.Contains(path, "include_body=") {
		t.Errorf("expected include_body to be omitted on default (true), got: %s", path)
	}
}

func TestWorkspaceContextNoBodyFlag(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       map[string]interface{}{"workspace": map[string]interface{}{"id": "7"}},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	workspaceContextLimit = 5
	workspaceContextNoBody = true
	workspaceContextMaxBodyChars = 500
	defer func() {
		workspaceContextLimit = 10
		workspaceContextNoBody = false
		workspaceContextMaxBodyChars = 500
	}()

	RunTestCommand(func() {
		workspaceContextCmd.Run(workspaceContextCmd, []string{"7"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	path := mock.GetCalls[0].Path
	if !strings.Contains(path, "include_body=false") {
		t.Errorf("expected include_body=false in path, got: %s", path)
	}
	if !strings.Contains(path, "limit=5") {
		t.Errorf("expected limit=5 in path, got: %s", path)
	}
}

func TestWorkspaceContextLimitOutOfRange(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	workspaceContextLimit = 99
	workspaceContextNoBody = false
	workspaceContextMaxBodyChars = 500
	defer func() {
		workspaceContextLimit = 10
		workspaceContextNoBody = false
		workspaceContextMaxBodyChars = 500
	}()

	RunTestCommand(func() {
		workspaceContextCmd.Run(workspaceContextCmd, []string{"5"})
	})

	if result.ExitCode != 2 {
		t.Errorf("expected exit code 2 (invalid args), got %d", result.ExitCode)
	}
	if len(mock.GetCalls) != 0 {
		t.Errorf("expected no Get calls on validation failure, got %d", len(mock.GetCalls))
	}
}

func TestWorkspaceContextMaxBodyCharsOutOfRange(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	workspaceContextLimit = 10
	workspaceContextNoBody = false
	workspaceContextMaxBodyChars = 50
	defer func() {
		workspaceContextLimit = 10
		workspaceContextNoBody = false
		workspaceContextMaxBodyChars = 500
	}()

	RunTestCommand(func() {
		workspaceContextCmd.Run(workspaceContextCmd, []string{"5"})
	})

	if result.ExitCode != 2 {
		t.Errorf("expected exit code 2 (invalid args), got %d", result.ExitCode)
	}
}
