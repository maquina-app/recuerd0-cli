package commands

import (
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
)

func TestMemoryVersionCreate(t *testing.T) {
	mock := NewMockClient()
	mock.PostResponse = &client.APIResponse{
		StatusCode: 201,
		Data:       map[string]interface{}{"id": "200", "memory_id": "42"},
	}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryVersionCreateWorkspace = ""
	memoryVersionCreateTitle = "Updated Title"
	memoryVersionCreateContent = "New content"
	memoryVersionCreateTags = "v2,updated"
	defer func() {
		memoryVersionCreateWorkspace = ""
		memoryVersionCreateTitle = ""
		memoryVersionCreateContent = ""
		memoryVersionCreateSource = ""
		memoryVersionCreateTags = ""
	}()

	RunTestCommand(func() {
		memoryVersionCreateCmd.Run(memoryVersionCreateCmd, []string{"42"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if !result.Response.Success {
		t.Error("expected success response")
	}
	if len(mock.PostCalls) != 1 {
		t.Fatalf("expected 1 Post call, got %d", len(mock.PostCalls))
	}
	if mock.PostCalls[0].Path != "/workspaces/5/memories/42/versions" {
		t.Errorf("unexpected path: %s", mock.PostCalls[0].Path)
	}

	body, ok := mock.PostCalls[0].Body.(map[string]interface{})
	if !ok {
		t.Fatal("expected body to be a map")
	}
	ver, ok := body["version"].(map[string]interface{})
	if !ok {
		t.Fatal("expected version key in body")
	}
	if ver["title"] != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got %v", ver["title"])
	}
}

func TestVersionCreateWithCategory(t *testing.T) {
	mock := NewMockClient()
	mock.PostResponse = &client.APIResponse{StatusCode: 201, Data: map[string]interface{}{"id": "200"}}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryVersionCreateTitle = "V"
	memoryVersionCreateCategory = "discovery"
	defer func() {
		memoryVersionCreateTitle = ""
		memoryVersionCreateCategory = ""
	}()

	RunTestCommand(func() { memoryVersionCreateCmd.Run(memoryVersionCreateCmd, []string{"42"}) })

	if result.ExitCode != 0 {
		t.Fatalf("expected 0, got %d", result.ExitCode)
	}
	body := mock.PostCalls[0].Body.(map[string]interface{})
	ver := body["version"].(map[string]interface{})
	if ver["category"] != "discovery" {
		t.Errorf("expected discovery, got %v", ver["category"])
	}
}

func TestVersionCreateInheritsCategory(t *testing.T) {
	mock := NewMockClient()
	mock.PostResponse = &client.APIResponse{StatusCode: 201, Data: map[string]interface{}{"id": "200"}}

	result := SetTestMode(mock)
	SetTestConfigFull("tok_test", "https://api.example.com", "5")
	defer ResetTestMode()

	memoryVersionCreateTitle = "V"
	memoryVersionCreateCategory = ""
	defer func() { memoryVersionCreateTitle = "" }()

	RunTestCommand(func() { memoryVersionCreateCmd.Run(memoryVersionCreateCmd, []string{"42"}) })

	if result.ExitCode != 0 {
		t.Fatalf("expected 0, got %d", result.ExitCode)
	}
	body := mock.PostCalls[0].Body.(map[string]interface{})
	ver := body["version"].(map[string]interface{})
	if _, has := ver["category"]; has {
		t.Errorf("expected category to be omitted, got %v", ver["category"])
	}
}

func TestMemoryVersionCreate_NoAuth(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("", "https://api.example.com")
	defer ResetTestMode()

	RunTestCommand(func() {
		memoryVersionCreateCmd.Run(memoryVersionCreateCmd, []string{"42"})
	})

	if result.Response.Success {
		t.Error("expected error response")
	}
}
