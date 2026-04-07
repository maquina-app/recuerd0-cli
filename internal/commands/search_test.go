package commands

import (
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
	"github.com/maquina/recuerd0-cli/internal/errors"
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
	if mock.GetCalls[0].Path != "/search?q=golang+patterns" {
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
	if mock.GetCalls[0].Path != "/search?q=test&workspace_id=5" {
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
	if mock.GetCalls[0].Path != "/search?page=3&q=query" {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestSearchWithCategoryFilter(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: []interface{}{}}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	searchCategory = "decision"
	defer func() { searchCategory = "" }()

	RunTestCommand(func() { searchCmd.Run(searchCmd, []string{"test"}) })

	if result.ExitCode != 0 {
		t.Fatalf("expected 0, got %d", result.ExitCode)
	}
	if !strings.Contains(mock.GetCalls[0].Path, "category=decision") {
		t.Errorf("expected category=decision, got: %s", mock.GetCalls[0].Path)
	}
}

func TestSearchInvalidCategory(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	searchCategory = "bogus"
	defer func() { searchCategory = "" }()

	RunTestCommand(func() { searchCmd.Run(searchCmd, []string{"test"}) })

	if result.ExitCode != errors.ExitInvalidArgs {
		t.Errorf("expected exit %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
	}
	if len(mock.GetCalls) != 0 {
		t.Errorf("expected no Get calls, got %d", len(mock.GetCalls))
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
