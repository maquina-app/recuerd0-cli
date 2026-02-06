package commands

import (
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
)

func TestSearch(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       []interface{}{map[string]interface{}{"id": "1", "title": "Result"}},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	searchWorkspace = ""
	searchPage = ""

	RunTestCommand(func() {
		searchCmd.Run(searchCmd, []string{"golang patterns"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if !result.Response.Success {
		t.Error("expected success response")
	}
	if mock.GetCalls[0].Path != "/api/v1/search?q=golang patterns" {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestSearch_WithWorkspace(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       []interface{}{},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	searchWorkspace = "5"
	searchPage = ""
	defer func() { searchWorkspace = "" }()

	RunTestCommand(func() {
		searchCmd.Run(searchCmd, []string{"test"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if mock.GetCalls[0].Path != "/api/v1/search?q=test&workspace_id=5" {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestSearch_WithPage(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       []interface{}{},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	searchWorkspace = ""
	searchPage = "3"
	defer func() { searchPage = "" }()

	RunTestCommand(func() {
		searchCmd.Run(searchCmd, []string{"query"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if mock.GetCalls[0].Path != "/api/v1/search?q=query&page=3" {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestSearch_NoAuth(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("", "https://api.example.com")
	defer ResetTestMode()

	RunTestCommand(func() {
		searchCmd.Run(searchCmd, []string{"test"})
	})

	if result.Response.Success {
		t.Error("expected error response")
	}
}
