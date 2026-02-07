package commands

import (
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
	"github.com/maquina/recuerd0-cli/internal/errors"
)

func TestWorkspaceList(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       []interface{}{map[string]interface{}{"id": "1", "name": "My Workspace"}},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	RunTestCommand(func() {
		workspaceListCmd.Run(workspaceListCmd, []string{})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if !result.Response.Success {
		t.Error("expected success response")
	}
	if result.Response.Summary != "1 workspace(s)" {
		t.Errorf("expected summary '1 workspace(s)', got %q", result.Response.Summary)
	}
	if len(mock.GetCalls) != 1 {
		t.Errorf("expected 1 Get call, got %d", len(mock.GetCalls))
	}
	if mock.GetCalls[0].Path != "/workspaces" {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestWorkspaceList_WithPage(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       []interface{}{},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	workspaceListPage = "2"
	defer func() { workspaceListPage = "" }()

	RunTestCommand(func() {
		workspaceListCmd.Run(workspaceListCmd, []string{})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if mock.GetCalls[0].Path != "/workspaces?page=2" {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestWorkspaceList_NoAuth(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("", "https://api.example.com")
	defer ResetTestMode()

	RunTestCommand(func() {
		workspaceListCmd.Run(workspaceListCmd, []string{})
	})

	if result.Response.Success {
		t.Error("expected error response")
	}
	if result.ExitCode != errors.ExitAuthFailure {
		t.Errorf("expected exit code %d, got %d", errors.ExitAuthFailure, result.ExitCode)
	}
}

func TestWorkspaceShow(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       map[string]interface{}{"id": "5", "name": "Test"},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	RunTestCommand(func() {
		workspaceShowCmd.Run(workspaceShowCmd, []string{"5"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if mock.GetCalls[0].Path != "/workspaces/5" {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestWorkspaceCreate(t *testing.T) {
	mock := NewMockClient()
	mock.PostResponse = &client.APIResponse{
		StatusCode: 201,
		Data:       map[string]interface{}{"id": "10", "name": "New WS"},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	workspaceCreateName = "New WS"
	workspaceCreateDesc = "A description"
	defer func() { workspaceCreateName = ""; workspaceCreateDesc = "" }()

	RunTestCommand(func() {
		workspaceCreateCmd.Run(workspaceCreateCmd, []string{})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if len(mock.PostCalls) != 1 {
		t.Fatalf("expected 1 Post call, got %d", len(mock.PostCalls))
	}
	if mock.PostCalls[0].Path != "/workspaces" {
		t.Errorf("unexpected path: %s", mock.PostCalls[0].Path)
	}
}

func TestWorkspaceCreate_MissingName(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	workspaceCreateName = ""
	defer func() { workspaceCreateName = "" }()

	RunTestCommand(func() {
		workspaceCreateCmd.Run(workspaceCreateCmd, []string{})
	})

	if result.Response.Success {
		t.Error("expected error response")
	}
}

func TestWorkspaceUpdate(t *testing.T) {
	mock := NewMockClient()
	mock.PatchResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       map[string]interface{}{"id": "5", "name": "Updated"},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	workspaceUpdateName = "Updated"
	defer func() { workspaceUpdateName = ""; workspaceUpdateDesc = "" }()

	RunTestCommand(func() {
		workspaceUpdateCmd.Run(workspaceUpdateCmd, []string{"5"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if mock.PatchCalls[0].Path != "/workspaces/5" {
		t.Errorf("unexpected path: %s", mock.PatchCalls[0].Path)
	}
}

func TestWorkspaceUpdate_NoFields(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	workspaceUpdateName = ""
	workspaceUpdateDesc = ""

	RunTestCommand(func() {
		workspaceUpdateCmd.Run(workspaceUpdateCmd, []string{"5"})
	})

	if result.Response.Success {
		t.Error("expected error when no fields specified")
	}
}
