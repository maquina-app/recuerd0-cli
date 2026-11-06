package commands

import (
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
)

func TestIsValidInclude(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"obsolete", true},
		{"OBSOLETE", true},
		{" obsolete ", true},
		{"obsolete,", true},
		{",", true},
		{"obsolte", false},
		{"obsolete,future_token", false},
		{"deprecated", false},
	}
	for _, c := range cases {
		if got := IsValidInclude(c.in); got != c.want {
			t.Errorf("IsValidInclude(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestUnknownIncludeTokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"obsolete", nil},
		{"bogus", []string{"bogus"}},
		{"obsolete,bogus", []string{"bogus"}},
		{"a,b", []string{"a", "b"}},
	}
	for _, c := range cases {
		got := UnknownIncludeTokens(c.in)
		if len(got) != len(c.want) {
			t.Errorf("UnknownIncludeTokens(%q) = %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("UnknownIncludeTokens(%q) = %v, want %v", c.in, got, c.want)
				break
			}
		}
	}
}

func TestIncludeParam(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"  ", ""},
		{"obsolete", "obsolete"},
		{" OBSOLETE ", "obsolete"},
		{"obsolete,,", "obsolete"},
	}
	for _, c := range cases {
		if got := includeParam(c.in); got != c.want {
			t.Errorf("includeParam(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCountObsoleteHidden(t *testing.T) {
	if got := countObsoleteHidden(map[string]interface{}{"obsolete_hidden": float64(3)}); got != 3 {
		t.Errorf("expected 3, got %d", got)
	}
	if got := countObsoleteHidden(map[string]interface{}{"obsolete_hidden": float64(0)}); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
	if got := countObsoleteHidden(map[string]interface{}{}); got != 0 {
		t.Errorf("expected 0 for missing key, got %d", got)
	}
	if got := countObsoleteHidden([]interface{}{}); got != 0 {
		t.Errorf("expected 0 for non-object, got %d", got)
	}
}

func TestSearchWithIncludeObsolete(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: map[string]interface{}{
		"total_results": float64(1), "obsolete_hidden": float64(0),
	}}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	searchInclude = "obsolete"
	defer func() { searchInclude = "" }()

	RunTestCommand(func() { searchCmd.Run(searchCmd, []string{"auth"}) })

	if result.ExitCode != 0 {
		t.Fatalf("expected 0, got %d", result.ExitCode)
	}
	if !strings.Contains(mock.GetCalls[0].Path, "include=obsolete") {
		t.Errorf("expected include=obsolete, got: %s", mock.GetCalls[0].Path)
	}
}

func TestSearchOmitsIncludeByDefault(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: map[string]interface{}{}}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	RunTestCommand(func() { searchCmd.Run(searchCmd, []string{"auth"}) })

	if result.ExitCode != 0 {
		t.Fatalf("expected 0, got %d", result.ExitCode)
	}
	if strings.Contains(mock.GetCalls[0].Path, "include=") {
		t.Errorf("expected no include param, got: %s", mock.GetCalls[0].Path)
	}
}

// An unrecognized token matches nothing, so the honest answer is an empty
// result set — not an error, and not the unwidened list.
func TestSearchUnknownIncludeReturnsEmpty(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	searchInclude = "bogus"
	defer func() { searchInclude = "" }()

	RunTestCommand(func() { searchCmd.Run(searchCmd, []string{"auth"}) })

	if result.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", result.ExitCode)
	}
	if !result.Response.Success {
		t.Error("expected success response")
	}
	assertEmptyList(t, result.Response.Data)
	if !strings.Contains(result.Response.Summary, "bogus") {
		t.Errorf("expected summary to name the token, got: %q", result.Response.Summary)
	}
	if len(mock.GetCalls) != 0 {
		t.Errorf("expected no Get calls, got %d", len(mock.GetCalls))
	}
}

// A valid token alongside an invalid one is still an unrecognized request.
func TestSearchPartiallyUnknownIncludeReturnsEmpty(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	searchInclude = "obsolete,bogus"
	defer func() { searchInclude = "" }()

	RunTestCommand(func() { searchCmd.Run(searchCmd, []string{"auth"}) })

	if result.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", result.ExitCode)
	}
	assertEmptyList(t, result.Response.Data)
	if len(mock.GetCalls) != 0 {
		t.Errorf("expected no Get calls, got %d", len(mock.GetCalls))
	}
}

func assertEmptyList(t *testing.T, data interface{}) {
	t.Helper()
	items, ok := data.([]interface{})
	if !ok {
		t.Fatalf("expected a list payload, got %T", data)
	}
	if len(items) != 0 {
		t.Errorf("expected empty payload, got %d item(s)", len(items))
	}
}

// The server reports what it withheld; the summary says so and a breadcrumb
// offers the way back.
func TestSearchReportsObsoleteHidden(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: map[string]interface{}{
		"total_results": float64(2), "obsolete_hidden": float64(1),
	}}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	RunTestCommand(func() { searchCmd.Run(searchCmd, []string{"auth"}) })

	if result.ExitCode != 0 {
		t.Fatalf("expected 0, got %d", result.ExitCode)
	}
	if !strings.Contains(result.Response.Summary, "1 obsolete hidden") {
		t.Errorf("expected summary to report hidden count, got: %q", result.Response.Summary)
	}
	found := false
	for _, b := range result.Response.Breadcrumbs {
		if b.Action == "include-obsolete" {
			found = true
			if !strings.Contains(b.Cmd, "--include obsolete") {
				t.Errorf("unexpected breadcrumb command: %s", b.Cmd)
			}
		}
	}
	if !found {
		t.Error("expected an include-obsolete breadcrumb")
	}
}

func TestMemoryListWithIncludeObsolete(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: []interface{}{}}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	memoryListWorkspace = "1"
	memoryListInclude = "obsolete"
	defer func() { memoryListWorkspace = ""; memoryListInclude = "" }()

	RunTestCommand(func() { memoryListCmd.Run(memoryListCmd, []string{}) })

	if result.ExitCode != 0 {
		t.Fatalf("expected 0, got %d", result.ExitCode)
	}
	if !strings.Contains(mock.GetCalls[0].Path, "include=obsolete") {
		t.Errorf("expected include=obsolete, got: %s", mock.GetCalls[0].Path)
	}
}

func TestMemoryListUnknownIncludeReturnsEmpty(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	memoryListWorkspace = "1"
	memoryListInclude = "bogus"
	defer func() { memoryListWorkspace = ""; memoryListInclude = "" }()

	RunTestCommand(func() { memoryListCmd.Run(memoryListCmd, []string{}) })

	if result.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", result.ExitCode)
	}
	if !result.Response.Success {
		t.Error("expected success response")
	}
	assertEmptyList(t, result.Response.Data)
	if len(mock.GetCalls) != 0 {
		t.Errorf("expected no Get calls, got %d", len(mock.GetCalls))
	}
}

func TestWorkspaceContextWithIncludeObsolete(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: map[string]interface{}{}}

	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	workspaceContextLimit = 10
	workspaceContextMaxBodyChars = 500
	workspaceContextInclude = "obsolete"
	defer func() { workspaceContextInclude = "" }()

	RunTestCommand(func() { workspaceContextCmd.Run(workspaceContextCmd, []string{"1"}) })

	if result.ExitCode != 0 {
		t.Fatalf("expected 0, got %d", result.ExitCode)
	}
	if !strings.Contains(mock.GetCalls[0].Path, "include=obsolete") {
		t.Errorf("expected include=obsolete, got: %s", mock.GetCalls[0].Path)
	}
}

func TestWorkspaceContextUnknownIncludeReturnsEmpty(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfig("tok_test", "https://api.example.com")
	defer ResetTestMode()

	workspaceContextLimit = 10
	workspaceContextMaxBodyChars = 500
	workspaceContextInclude = "bogus"
	defer func() { workspaceContextInclude = "" }()

	RunTestCommand(func() { workspaceContextCmd.Run(workspaceContextCmd, []string{"1"}) })

	if result.ExitCode != 0 {
		t.Errorf("expected exit 0, got %d", result.ExitCode)
	}
	if !result.Response.Success {
		t.Error("expected success response")
	}
	payload, ok := result.Response.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected an object payload, got %T", result.Response.Data)
	}
	if len(payload) != 0 {
		t.Errorf("expected empty payload, got %d key(s)", len(payload))
	}
	if len(mock.GetCalls) != 0 {
		t.Errorf("expected no Get calls, got %d", len(mock.GetCalls))
	}
}

func TestIgnoredWordsReadsTheSearchPayload(t *testing.T) {
	got := ignoredWords(map[string]interface{}{"ignored_words": []interface{}{"to", "in"}})
	if len(got) != 2 || got[0] != "to" || got[1] != "in" {
		t.Errorf("expected the words back, got %v", got)
	}
	if got := ignoredWords(map[string]interface{}{}); len(got) != 0 {
		t.Errorf("absent means none, got %v", got)
	}
	if got := ignoredWords([]interface{}{}); len(got) != 0 {
		t.Errorf("an older server's list payload means none, got %v", got)
	}
}

func TestSearchPassesInactiveThrough(t *testing.T) {
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: map[string]interface{}{"results": []interface{}{}}}
	result := SetTestMode(mock)
	SetTestConfig("tok", "https://api.example.com")
	defer ResetTestMode()

	searchInclude = "obsolete,inactive"
	defer func() { searchInclude = "" }()

	RunTestCommand(func() { searchCmd.Run(searchCmd, []string{"deploy"}) })

	if result.ExitCode != 0 || len(mock.GetCalls) != 1 {
		t.Fatalf("expected one request, got exit %d and %d calls", result.ExitCode, len(mock.GetCalls))
	}
	if !strings.Contains(mock.GetCalls[0].Path, "include=obsolete%2Cinactive") {
		t.Errorf("include was not sent: %s", mock.GetCalls[0].Path)
	}
}
