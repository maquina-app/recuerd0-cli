package commands

import (
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
)

func TestMemoryLinkList(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data:       []interface{}{map[string]interface{}{"id": 99}},
	}

	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryLinkListWorkspace = ""
	defer func() { memoryLinkListWorkspace = "" }()

	RunTestCommand(func() {
		memoryLinkListCmd.Run(memoryLinkListCmd, []string{"42"})
	})

	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if len(mock.GetCalls) != 1 {
		t.Fatalf("expected 1 Get call, got %d", len(mock.GetCalls))
	}
	if mock.GetCalls[0].Path != "/workspaces/ws1/memories/42/links" {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestMemoryLinkListInvalidMemoryID(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryLinkListWorkspace = ""
	defer func() { memoryLinkListWorkspace = "" }()

	RunTestCommand(func() {
		memoryLinkListCmd.Run(memoryLinkListCmd, []string{"abc"})
	})

	if result.ExitCode != 2 {
		t.Errorf("expected exit 2, got %d", result.ExitCode)
	}
	if len(mock.GetCalls) != 0 {
		t.Errorf("expected no Get calls, got %d", len(mock.GetCalls))
	}
}

func TestMemoryLinkAdd(t *testing.T) {
	mock := NewMockClient()
	mock.PostResponse = &client.APIResponse{StatusCode: 201, Data: map[string]interface{}{"ok": true}}

	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryLinkAddWorkspace = ""
	memoryLinkAddTo = 99
	defer func() {
		memoryLinkAddWorkspace = ""
		memoryLinkAddTo = 0
	}()

	RunTestCommand(func() {
		memoryLinkAddCmd.Run(memoryLinkAddCmd, []string{"42"})
	})

	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if len(mock.PostCalls) != 1 {
		t.Fatalf("expected 1 Post call, got %d", len(mock.PostCalls))
	}
	if mock.PostCalls[0].Path != "/workspaces/ws1/memories/42/links" {
		t.Errorf("unexpected path: %s", mock.PostCalls[0].Path)
	}
	body, ok := mock.PostCalls[0].Body.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map body, got %T", mock.PostCalls[0].Body)
	}
	if body["to_memory_id"] != 99 {
		t.Errorf("expected to_memory_id=99, got %v", body["to_memory_id"])
	}
}

func TestMemoryLinkAddMissingTo(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryLinkAddWorkspace = ""
	memoryLinkAddTo = 0
	defer func() {
		memoryLinkAddWorkspace = ""
		memoryLinkAddTo = 0
	}()

	RunTestCommand(func() {
		memoryLinkAddCmd.Run(memoryLinkAddCmd, []string{"42"})
	})

	if result.ExitCode != 2 {
		t.Errorf("expected exit 2, got %d", result.ExitCode)
	}
	if len(mock.PostCalls) != 0 {
		t.Errorf("expected no Post calls, got %d", len(mock.PostCalls))
	}
}

func TestMemoryLinkAddSelfLink(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryLinkAddWorkspace = ""
	memoryLinkAddTo = 42
	defer func() {
		memoryLinkAddWorkspace = ""
		memoryLinkAddTo = 0
	}()

	RunTestCommand(func() {
		memoryLinkAddCmd.Run(memoryLinkAddCmd, []string{"42"})
	})

	if result.ExitCode != 2 {
		t.Errorf("expected exit 2, got %d", result.ExitCode)
	}
	if len(mock.PostCalls) != 0 {
		t.Errorf("expected no Post calls, got %d", len(mock.PostCalls))
	}
}

func TestMemoryLinkAddInvalidMemoryID(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryLinkAddWorkspace = ""
	memoryLinkAddTo = 99
	defer func() {
		memoryLinkAddWorkspace = ""
		memoryLinkAddTo = 0
	}()

	RunTestCommand(func() {
		memoryLinkAddCmd.Run(memoryLinkAddCmd, []string{"nope"})
	})

	if result.ExitCode != 2 {
		t.Errorf("expected exit 2, got %d", result.ExitCode)
	}
	if len(mock.PostCalls) != 0 {
		t.Errorf("expected no Post calls, got %d", len(mock.PostCalls))
	}
}

func TestMemoryLinkRemove(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryLinkRemoveWorkspace = ""
	memoryLinkRemoveTo = 99
	defer func() {
		memoryLinkRemoveWorkspace = ""
		memoryLinkRemoveTo = 0
	}()

	RunTestCommand(func() {
		memoryLinkRemoveCmd.Run(memoryLinkRemoveCmd, []string{"42"})
	})

	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if len(mock.DeleteCalls) != 1 {
		t.Fatalf("expected 1 Delete call, got %d", len(mock.DeleteCalls))
	}
	if mock.DeleteCalls[0].Path != "/workspaces/ws1/memories/42/links/99" {
		t.Errorf("unexpected path: %s", mock.DeleteCalls[0].Path)
	}
}

func TestMemoryLinkRemoveMissingTo(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryLinkRemoveWorkspace = ""
	memoryLinkRemoveTo = 0
	defer func() {
		memoryLinkRemoveWorkspace = ""
		memoryLinkRemoveTo = 0
	}()

	RunTestCommand(func() {
		memoryLinkRemoveCmd.Run(memoryLinkRemoveCmd, []string{"42"})
	})

	if result.ExitCode != 2 {
		t.Errorf("expected exit 2, got %d", result.ExitCode)
	}
	if len(mock.DeleteCalls) != 0 {
		t.Errorf("expected no Delete calls, got %d", len(mock.DeleteCalls))
	}
}
