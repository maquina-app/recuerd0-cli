package commands

import (
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
)

func memoryReadResetHead() {
	memoryReadHeadWorkspace = ""
	memoryReadHeadLines = 20
}

func memoryReadResetTail() {
	memoryReadTailWorkspace = ""
	memoryReadTailLines = 20
}

func memoryReadResetLines() {
	memoryReadLinesWorkspace = ""
	memoryReadLinesStart = 0
	memoryReadLinesEnd = 0
}

func memoryReadResetGrep() {
	memoryReadGrepWorkspace = ""
	memoryReadGrepContext = 0
	memoryReadGrepBefore = -1
	memoryReadGrepAfter = -1
}

func TestMemoryReadHeadDefault(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data: map[string]interface{}{
			"id": 42,
			"content": map[string]interface{}{
				"total_lines": float64(100),
				"line_start":  float64(1),
				"line_end":    float64(20),
				"body":        "...",
			},
		},
	}

	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryReadResetHead()
	defer memoryReadResetHead()

	RunTestCommand(func() {
		memoryReadHeadCmd.Run(memoryReadHeadCmd, []string{"42"})
	})

	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if len(mock.GetCalls) != 1 {
		t.Fatalf("expected 1 Get call, got %d", len(mock.GetCalls))
	}
	if !strings.Contains(mock.GetCalls[0].Path, "line_start=1&line_end=20") {
		t.Errorf("expected default line_start=1&line_end=20, got %s", mock.GetCalls[0].Path)
	}
}

func TestMemoryReadHeadExplicitLines(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: map[string]interface{}{}}

	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryReadResetHead()
	memoryReadHeadLines = 5
	defer memoryReadResetHead()

	RunTestCommand(func() {
		memoryReadHeadCmd.Run(memoryReadHeadCmd, []string{"42"})
	})

	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if !strings.Contains(mock.GetCalls[0].Path, "line_start=1&line_end=5") {
		t.Errorf("expected line_end=5, got %s", mock.GetCalls[0].Path)
	}
}

func TestMemoryReadHeadInvalidLines(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryReadResetHead()
	memoryReadHeadLines = 0
	defer memoryReadResetHead()

	RunTestCommand(func() {
		memoryReadHeadCmd.Run(memoryReadHeadCmd, []string{"42"})
	})

	if result.ExitCode != 2 {
		t.Errorf("expected exit 2, got %d", result.ExitCode)
	}
	if len(mock.GetCalls) != 0 {
		t.Errorf("expected no Get calls, got %d", len(mock.GetCalls))
	}
}

func TestMemoryReadTail(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponses = []*client.APIResponse{
		{
			StatusCode: 200,
			Data: map[string]interface{}{
				"id": 42,
				"content": map[string]interface{}{
					"total_lines": float64(100),
					"line_start":  float64(1),
					"line_end":    float64(1),
				},
			},
		},
		{
			StatusCode: 200,
			Data: map[string]interface{}{
				"id": 42,
				"content": map[string]interface{}{
					"total_lines": float64(100),
					"line_start":  float64(81),
					"line_end":    float64(100),
					"body":        "...",
				},
			},
		},
	}

	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryReadResetTail()
	defer memoryReadResetTail()

	RunTestCommand(func() {
		memoryReadTailCmd.Run(memoryReadTailCmd, []string{"42"})
	})

	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if len(mock.GetCalls) != 2 {
		t.Fatalf("expected 2 Get calls, got %d", len(mock.GetCalls))
	}
	if !strings.Contains(mock.GetCalls[0].Path, "line_start=1&line_end=1") {
		t.Errorf("first call should probe with line_end=1, got %s", mock.GetCalls[0].Path)
	}
	if !strings.Contains(mock.GetCalls[1].Path, "line_start=81&line_end=100") {
		t.Errorf("second call should be line_start=81&line_end=100, got %s", mock.GetCalls[1].Path)
	}
}

func TestMemoryReadTailInvalidLines(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryReadResetTail()
	memoryReadTailLines = 0
	defer memoryReadResetTail()

	RunTestCommand(func() {
		memoryReadTailCmd.Run(memoryReadTailCmd, []string{"42"})
	})

	if result.ExitCode != 2 {
		t.Errorf("expected exit 2, got %d", result.ExitCode)
	}
	if len(mock.GetCalls) != 0 {
		t.Errorf("expected no Get calls, got %d", len(mock.GetCalls))
	}
}

func TestMemoryReadLines(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: map[string]interface{}{}}

	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryReadResetLines()
	memoryReadLinesStart = 10
	memoryReadLinesEnd = 20
	defer memoryReadResetLines()

	RunTestCommand(func() {
		memoryReadLinesCmd.Run(memoryReadLinesCmd, []string{"42"})
	})

	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if !strings.Contains(mock.GetCalls[0].Path, "line_start=10&line_end=20") {
		t.Errorf("unexpected path: %s", mock.GetCalls[0].Path)
	}
}

func TestMemoryReadLinesInvertedRange(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryReadResetLines()
	memoryReadLinesStart = 5
	memoryReadLinesEnd = 3
	defer memoryReadResetLines()

	RunTestCommand(func() {
		memoryReadLinesCmd.Run(memoryReadLinesCmd, []string{"42"})
	})

	if result.ExitCode != 2 {
		t.Errorf("expected exit 2, got %d", result.ExitCode)
	}
	if len(mock.GetCalls) != 0 {
		t.Errorf("expected no Get calls, got %d", len(mock.GetCalls))
	}
}

func TestMemoryReadLinesMissingFlags(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryReadResetLines()
	// Both default to 0; --start < 1 should fail validation.
	defer memoryReadResetLines()

	RunTestCommand(func() {
		memoryReadLinesCmd.Run(memoryReadLinesCmd, []string{"42"})
	})

	if result.ExitCode != 2 {
		t.Errorf("expected exit 2, got %d", result.ExitCode)
	}
	if len(mock.GetCalls) != 0 {
		t.Errorf("expected no Get calls, got %d", len(mock.GetCalls))
	}
}

func TestMemoryReadGrep(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{
		StatusCode: 200,
		Data: map[string]interface{}{
			"id": 42,
			"content": map[string]interface{}{
				"total_lines": float64(100),
				"matches": []interface{}{
					map[string]interface{}{
						"line_number":    float64(12),
						"line":           "TODO: fix",
						"context_before": []interface{}{},
						"context_after":  []interface{}{},
					},
					map[string]interface{}{
						"line_number":    float64(40),
						"line":           "TODO: refactor",
						"context_before": []interface{}{},
						"context_after":  []interface{}{},
					},
				},
			},
		},
	}

	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryReadResetGrep()
	defer memoryReadResetGrep()

	RunTestCommand(func() {
		memoryReadGrepCmd.Run(memoryReadGrepCmd, []string{"42", "TODO"})
	})

	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if len(mock.GetCalls) != 1 {
		t.Fatalf("expected 1 Get call, got %d", len(mock.GetCalls))
	}
	path := mock.GetCalls[0].Path
	if !strings.Contains(path, "mode=grep") || !strings.Contains(path, "q=TODO") {
		t.Errorf("expected mode=grep&q=TODO in path, got %s", path)
	}
	if result.Response == nil {
		t.Fatalf("expected response to be captured")
	}
	bc := result.Response.Breadcrumbs
	if len(bc) != 2 {
		t.Fatalf("expected 2 breadcrumbs (one per match), got %d", len(bc))
	}
	if !strings.Contains(bc[0].Cmd, "--start 10 --end 17") {
		t.Errorf("expected first breadcrumb to suggest --start 10 --end 17 for match on line 12, got: %s", bc[0].Cmd)
	}
	if !strings.Contains(bc[1].Cmd, "--start 38 --end 45") {
		t.Errorf("expected second breadcrumb to suggest --start 38 --end 45 for match on line 40, got: %s", bc[1].Cmd)
	}
}

func TestMemoryReadGrepWithContext(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: map[string]interface{}{}}

	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "ws1")
	defer ResetTestMode()

	memoryReadResetGrep()
	memoryReadGrepContext = 2
	defer memoryReadResetGrep()

	RunTestCommand(func() {
		memoryReadGrepCmd.Run(memoryReadGrepCmd, []string{"42", "TODO"})
	})

	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", result.ExitCode)
	}
	if !strings.Contains(mock.GetCalls[0].Path, "context=2") {
		t.Errorf("expected context=2 in path, got %s", mock.GetCalls[0].Path)
	}
}
