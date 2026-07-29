package commands

import (
	"io"
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
	"github.com/maquina/recuerd0-cli/internal/errors"
)

func resetMemoryListGlobals() {
	memoryListWorkspace = ""
	memoryListPage = ""
	memoryListCategory = ""
	memoryListTags = ""
	memoryListSource = ""
}

func TestMemoryList(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       []interface{}{map[string]interface{}{"id": "1", "title": "Test Memory"}},
	}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	resetMemoryListGlobals()
	defer resetMemoryListGlobals()

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

	resetMemoryListGlobals()
	memoryListWorkspace = "99"
	defer resetMemoryListGlobals()

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

	resetMemoryListGlobals()
	defer resetMemoryListGlobals()

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

func TestMemoryCreateWithCategory(t *testing.T) {
	mock := NewMockClient()
	mock.PostResponse = &client.APIResponse{StatusCode: 201, Data: map[string]interface{}{"id": "1"}}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryCreateTitle = "T"
	memoryCreateContent = "C"
	memoryCreateCategory = "decision"
	defer func() {
		memoryCreateTitle = ""
		memoryCreateContent = ""
		memoryCreateCategory = ""
	}()

	RunTestCommand(func() { memoryCreateCmd.Run(memoryCreateCmd, []string{}) })

	if result.ExitCode != 0 {
		t.Fatalf("expected 0, got %d", result.ExitCode)
	}
	body := mock.PostCalls[0].Body.(map[string]interface{})
	memory := body["memory"].(map[string]interface{})
	if memory["category"] != "decision" {
		t.Errorf("expected category decision, got %v", memory["category"])
	}
}

func TestMemoryCreateWithoutCategory(t *testing.T) {
	mock := NewMockClient()
	mock.PostResponse = &client.APIResponse{StatusCode: 201, Data: map[string]interface{}{"id": "1"}}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryCreateTitle = "T"
	memoryCreateContent = "C"
	memoryCreateCategory = ""
	defer func() {
		memoryCreateTitle = ""
		memoryCreateContent = ""
	}()

	RunTestCommand(func() { memoryCreateCmd.Run(memoryCreateCmd, []string{}) })

	if result.ExitCode != 0 {
		t.Fatalf("expected 0, got %d", result.ExitCode)
	}
	body := mock.PostCalls[0].Body.(map[string]interface{})
	memory := body["memory"].(map[string]interface{})
	if _, has := memory["category"]; has {
		t.Errorf("expected category to be omitted, got %v", memory["category"])
	}
}

func TestMemoryCreateInvalidCategory(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryCreateTitle = "T"
	memoryCreateContent = "C"
	memoryCreateCategory = "nonsense"
	defer func() {
		memoryCreateTitle = ""
		memoryCreateContent = ""
		memoryCreateCategory = ""
	}()

	RunTestCommand(func() { memoryCreateCmd.Run(memoryCreateCmd, []string{}) })

	if result.ExitCode != errors.ExitInvalidArgs {
		t.Errorf("expected exit %d, got %d", errors.ExitInvalidArgs, result.ExitCode)
	}
	if len(mock.PostCalls) != 0 {
		t.Errorf("expected no Post calls, got %d", len(mock.PostCalls))
	}
}

func TestMemoryUpdateWithCategory(t *testing.T) {
	mock := NewMockClient()
	mock.PatchResponse = &client.APIResponse{StatusCode: 200, Data: map[string]interface{}{"id": "42"}}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryUpdateCategory = "preference"
	defer func() { memoryUpdateCategory = "" }()

	RunTestCommand(func() { memoryUpdateCmd.Run(memoryUpdateCmd, []string{"42"}) })

	if result.ExitCode != 0 {
		t.Fatalf("expected 0, got %d", result.ExitCode)
	}
	body := mock.PatchCalls[0].Body.(map[string]interface{})
	memory := body["memory"].(map[string]interface{})
	if memory["category"] != "preference" {
		t.Errorf("expected preference, got %v", memory["category"])
	}
}

func TestMemoryListFilters(t *testing.T) {
	tests := []struct {
		name     string
		page     string
		category string
		tags     string
		source   string
		wantPath string
	}{
		{
			name:     "no filters",
			wantPath: "/workspaces/5/memories",
		},
		{
			name:     "tags only",
			tags:     "fragua",
			wantPath: "/workspaces/5/memories?tags=fragua",
		},
		{
			name:     "source only",
			source:   "fragua-execution",
			wantPath: "/workspaces/5/memories?source=fragua-execution",
		},
		{
			name:     "combined filters",
			page:     "2",
			category: "decision",
			tags:     "fragua",
			source:   "fragua-execution",
			wantPath: "/workspaces/5/memories?page=2&category=decision&tags=fragua&source=fragua-execution",
		},
		{
			name:     "escaped user content",
			tags:     "a b",
			wantPath: "/workspaces/5/memories?tags=a+b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockClient()
			mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: []interface{}{}}

			result := SetTestMode(mock)
			SetTestConfigFull("tok_test", "https://api.example.com", "5")
			defer ResetTestMode()

			resetMemoryListGlobals()
			defer resetMemoryListGlobals()
			memoryListPage = tt.page
			memoryListCategory = tt.category
			memoryListTags = tt.tags
			memoryListSource = tt.source

			RunTestCommand(func() { memoryListCmd.Run(memoryListCmd, []string{}) })

			if result.ExitCode != 0 {
				t.Fatalf("expected 0, got %d", result.ExitCode)
			}
			if got := mock.GetCalls[0].Path; got != tt.wantPath {
				t.Errorf("request path = %q, want %q", got, tt.wantPath)
			}
		})
	}
}

func TestMemoryListFilterFlags(t *testing.T) {
	tests := []struct {
		name  string
		usage string
	}{
		{name: "tags", usage: "filter by tags (comma-separated)"},
		{name: "source", usage: "filter by source"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag := memoryListCmd.Flags().Lookup(tt.name)
			if flag == nil {
				t.Fatalf("--%s flag is not registered", tt.name)
			}
			if flag.DefValue != "" {
				t.Errorf("--%s default = %q, want empty string", tt.name, flag.DefValue)
			}
			if flag.Usage != tt.usage {
				t.Errorf("--%s usage = %q, want %q", tt.name, flag.Usage, tt.usage)
			}
		})
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
