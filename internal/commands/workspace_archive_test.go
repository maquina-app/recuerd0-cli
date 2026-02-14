package commands

import (
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
)

func TestWorkspaceArchive(t *testing.T) {
	mock := NewMockClient()
	mock.PostResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       map[string]interface{}{"id": "5", "archived": true},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	RunTestCommand(func() {
		workspaceArchiveCmd.Run(workspaceArchiveCmd, []string{"5"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if len(mock.PostCalls) != 1 {
		t.Fatalf("expected 1 Post call, got %d", len(mock.PostCalls))
	}
	if mock.PostCalls[0].Path != "/workspaces/5/archive" {
		t.Errorf("unexpected path: %s", mock.PostCalls[0].Path)
	}
}

func TestWorkspaceUnarchive(t *testing.T) {
	mock := NewMockClient()
	mock.DeleteResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       map[string]interface{}{"id": "5", "archived": false},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	RunTestCommand(func() {
		workspaceUnarchiveCmd.Run(workspaceUnarchiveCmd, []string{"5"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if len(mock.DeleteCalls) != 1 {
		t.Fatalf("expected 1 Delete call, got %d", len(mock.DeleteCalls))
	}
	if mock.DeleteCalls[0].Path != "/workspaces/5/archive" {
		t.Errorf("unexpected path: %s", mock.DeleteCalls[0].Path)
	}
}
