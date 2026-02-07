package commands

import (
	"io"
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
	"github.com/maquina/recuerd0-cli/internal/errors"
)

func TestMemoryList(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       []interface{}{map[string]interface{}{"id": "1", "title": "Test Memory"}},
	}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryListWorkspace = ""
	defer func() { memoryListWorkspace = ""; memoryListPage = "" }()

	RunTestCommand(func() {
		memoryListCmd.Run(memoryListCmd, []string{})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if mock.GetCalls[0].Path != "/workspaces/5/memories" {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestMemoryList_WithExplicitWorkspace(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       []interface{}{},
	}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	memoryListWorkspace = "99"
	defer func() { memoryListWorkspace = "" }()

	RunTestCommand(func() {
		memoryListCmd.Run(memoryListCmd, []string{})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if mock.GetCalls[0].Path != "/workspaces/99/memories" {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestMemoryList_NoWorkspace(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	memoryListWorkspace = ""

	RunTestCommand(func() {
		memoryListCmd.Run(memoryListCmd, []string{})
	})

	if result.Response.Success {
		t.Error("expected error when no workspace")
	}
	if result.ExitCode != errors.ExitInvalidArgs {
		t.Errorf("expected exit code %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
	}
}

func TestMemoryShow(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       map[string]interface{}{"id": "42", "title": "Test"},
	}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryShowWorkspace = ""

	RunTestCommand(func() {
		memoryShowCmd.Run(memoryShowCmd, []string{"42"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if mock.GetCalls[0].Path != "/workspaces/5/memories/42" {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestMemoryCreate(t *testing.T) {
	mock := NewMockClient()
	mock.PostResponse = &client.APIResponse{
		StatusCode: 201,
		Data:       map[string]interface{}{"id": "100", "title": "New Memory"},
	}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryCreateWorkspace = ""
	memoryCreateTitle = "New Memory"
	memoryCreateContent = "Some content"
	memoryCreateSource = "claude"
	memoryCreateTags = "ai,coding"
	defer func() {
		memoryCreateWorkspace = ""
		memoryCreateTitle = ""
		memoryCreateContent = ""
		memoryCreateSource = ""
		memoryCreateTags = ""
	}()

	RunTestCommand(func() {
		memoryCreateCmd.Run(memoryCreateCmd, []string{})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if mock.PostCalls[0].Path != "/workspaces/5/memories" {
		t.Errorf("unexpected path: %s", mock.PostCalls[0].Path)
	}

	// Verify body structure
	body, ok := mock.PostCalls[0].Body.(map[string]interface{})
	if !ok {
		t.Fatal("expected body to be a map")
	}
	memory, ok := body["memory"].(map[string]interface{})
	if !ok {
		t.Fatal("expected memory key in body")
	}
	if memory["title"] != "New Memory" {
		t.Errorf("expected title 'New Memory', got %v", memory["title"])
	}
	tags, ok := memory["tags"].([]string)
	if !ok {
		t.Fatal("expected tags to be []string")
	}
	if len(tags) != 2 || tags[0] != "ai" || tags[1] != "coding" {
		t.Errorf("unexpected tags: %v", tags)
	}
}

func TestMemoryCreate_Stdin(t *testing.T) {
	mock := NewMockClient()
	mock.PostResponse = &client.APIResponse{
		StatusCode: 201,
		Data:       map[string]interface{}{"id": "101"},
	}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	// Override stdin reader
	origReader := stdinReader
	stdinReader = func() io.Reader {
		return strings.NewReader("content from stdin")
	}
	defer func() { stdinReader = origReader }()

	memoryCreateWorkspace = ""
	memoryCreateTitle = "Stdin Test"
	memoryCreateContent = "-"
	defer func() {
		memoryCreateTitle = ""
		memoryCreateContent = ""
	}()

	RunTestCommand(func() {
		memoryCreateCmd.Run(memoryCreateCmd, []string{})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	body := mock.PostCalls[0].Body.(map[string]interface{})
	memory := body["memory"].(map[string]interface{})
	if memory["content"] != "content from stdin" {
		t.Errorf("expected stdin content, got %v", memory["content"])
	}
}

func TestMemoryUpdate(t *testing.T) {
	mock := NewMockClient()
	mock.PatchResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       map[string]interface{}{"id": "42", "title": "Updated"},
	}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryUpdateWorkspace = ""
	memoryUpdateTitle = "Updated"
	defer func() {
		memoryUpdateWorkspace = ""
		memoryUpdateTitle = ""
		memoryUpdateContent = ""
		memoryUpdateSource = ""
		memoryUpdateTags = ""
	}()

	RunTestCommand(func() {
		memoryUpdateCmd.Run(memoryUpdateCmd, []string{"42"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if mock.PatchCalls[0].Path != "/workspaces/5/memories/42" {
		t.Errorf("unexpected path: %s", mock.PatchCalls[0].Path)
	}
}

func TestMemoryUpdate_NoFields(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryUpdateWorkspace = ""
	memoryUpdateTitle = ""
	memoryUpdateContent = ""
	memoryUpdateSource = ""
	memoryUpdateTags = ""

	RunTestCommand(func() {
		memoryUpdateCmd.Run(memoryUpdateCmd, []string{"42"})
	})

	if result.Response.Success {
		t.Error("expected error when no fields specified")
	}
}

func TestMemoryDelete(t *testing.T) {
	mock := NewMockClient()
	mock.DeleteResponse = &client.APIResponse{StatusCode: 204}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryDeleteWorkspace = ""

	RunTestCommand(func() {
		memoryDeleteCmd.Run(memoryDeleteCmd, []string{"42"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if mock.DeleteCalls[0].Path != "/workspaces/5/memories/42" {
		t.Errorf("unexpected path: %s", mock.DeleteCalls[0].Path)
	}
}

func TestParseTags(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"ai,coding", []string{"ai", "coding"}},
		{"ai, coding, test", []string{"ai", "coding", "test"}},
		{"single", []string{"single"}},
		{"", []string{}},
	}

	for _, tt := range tests {
		got := parseTags(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("parseTags(%q) = %v, want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("parseTags(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}
