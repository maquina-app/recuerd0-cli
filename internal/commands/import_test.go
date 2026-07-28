package commands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
	clierrors "github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/importer"
	"github.com/maquina/recuerd0-cli/internal/response"
)

const importHelpSentence = "Import is propose → review → commit: `propose` writes a reviewable `import.plan.yaml` and never touches the server; `commit` executes the plan you approved."

func TestImportHelpContainsCanonicalLoopAndNoDryRun(t *testing.T) {
	if !strings.Contains(importCmd.Long, importHelpSentence) {
		t.Fatalf("import help changed:\n%s", importCmd.Long)
	}
	removedFlag := strings.Join([]string{"dry", "run"}, "-")
	if importCommitCmd.Flags().Lookup(removedFlag) != nil {
		t.Fatalf("%s must not be registered", removedFlag)
	}
	if got := importCommitCmd.Flags().Lookup("yes").Usage; got != "skip the confirmation prompt; required when not interactive" {
		t.Fatalf("--yes help = %q", got)
	}
}

func TestImportProposeGuidanceAndEnvelopeUseDisplayPath(t *testing.T) {
	tests := []struct {
		name        string
		planPath    func(root, cwd string) string
		displayPath func(root, cwd string) string
	}{
		{
			name: "default plan under working directory",
			planPath: func(root, cwd string) string {
				return "import.plan.yaml"
			},
			displayPath: func(root, cwd string) string {
				return "import.plan.yaml"
			},
		},
		{
			name: "custom plan under working directory",
			planPath: func(root, cwd string) string {
				return filepath.Join(cwd, "plans", "..", "plans", "custom.plan.yaml")
			},
			displayPath: func(root, cwd string) string {
				return filepath.Join("plans", "custom.plan.yaml")
			},
		},
		{
			name: "plan outside working directory",
			planPath: func(root, cwd string) string {
				return filepath.Join(root, "outside.plan.yaml")
			},
			displayPath: func(root, cwd string) string {
				return filepath.Join(root, "outside.plan.yaml")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			cwd := filepath.Join(root, "project")
			source := filepath.Join(cwd, "vault")
			if err := os.MkdirAll(source, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "one.md"), []byte("# One\n\nBody\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(cwd, "plans"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Chdir(cwd)

			planPath := tt.planPath(root, cwd)
			displayPath := tt.displayPath(root, cwd)
			var ttyStdout, nonTTYStdout []byte
			for _, pretty := range []bool{false, true} {
				ttyResponse, ttyStderr := runImportProposeGuidanceTest(t, source, planPath, true)
				nonTTYResponse, nonTTYStderr := runImportProposeGuidanceTest(t, source, planPath, false)

				ttyStdout = normalizedResponseJSON(t, ttyResponse, pretty)
				nonTTYStdout = normalizedResponseJSON(t, nonTTYResponse, pretty)
				if !bytes.Equal(ttyStdout, nonTTYStdout) {
					t.Fatalf("TTY changed stdout for pretty=%t:\nTTY: %s\nnon-TTY: %s", pretty, ttyStdout, nonTTYStdout)
				}
				if nonTTYStderr != "" {
					t.Fatalf("non-TTY stderr must be empty, got %q", nonTTYStderr)
				}

				wantGuidance := fmt.Sprintf(`Plan written to %s

Review it — the manifest lists one entry per file with the title, category and
tags it will use. Edit the file if something is wrong.

  recuerd0 import commit %s    execute after review
`, displayPath, displayPath)
				if ttyStderr != wantGuidance {
					t.Fatalf("unexpected TTY guidance:\ngot:\n%s\nwant:\n%s", ttyStderr, wantGuidance)
				}
				if strings.Count(ttyStderr, displayPath) != 2 {
					t.Fatalf("display path must appear twice, got %q", ttyStderr)
				}
				if ttyResponse.Location != displayPath ||
					ttyResponse.Summary != fmt.Sprintf("Import plan written to %s", displayPath) {
					t.Fatalf("unexpected envelope guidance: %#v", ttyResponse)
				}
				if len(ttyResponse.Breadcrumbs) != 2 ||
					ttyResponse.Breadcrumbs[0].Cmd != displayPath ||
					!strings.Contains(ttyResponse.Breadcrumbs[1].Cmd, displayPath) {
					t.Fatalf("unexpected breadcrumbs: %#v", ttyResponse.Breadcrumbs)
				}
				if _, ok := ttyResponse.Data.(importer.Digest); !ok {
					t.Fatalf("digest data changed type: %T", ttyResponse.Data)
				}
			}
		})
	}
}

func TestImportCommitPromptWordingIsActionAware(t *testing.T) {
	tests := []struct {
		name     string
		creates  int
		versions int
		want     string
	}{
		{"create only", 12, 0, "Create 12 memories in workspace 83 (Roasting notes)? [y/N] "},
		{"version only", 0, 3, "Create 3 versions in workspace 83 (Roasting notes)? [y/N] "},
		{"mixed", 12, 3, "Create 12 memories and 3 versions in workspace 83 (Roasting notes)? [y/N] "},
		{"singular", 1, 1, "Create 1 memory and 1 version in workspace 83 (Roasting notes)? [y/N] "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stderr := setImportCommandIO(t, "yes\n", true, false)
			if !confirmImport(tt.creates, tt.versions, 83, "Roasting notes") {
				t.Fatal("yes should confirm")
			}
			if got := strings.TrimSuffix(stderr.String(), "\n"); got != tt.want {
				t.Fatalf("prompt = %q, want %q", got, tt.want)
			}
			if strings.Contains(stderr.String(), "skip") {
				t.Fatalf("prompt must omit skipped rows: %q", stderr.String())
			}
		})
	}
}

func TestImportCommitInteractiveDeclineWritesNothingAndEmitsNoEnvelope(t *testing.T) {
	planPath := writeCommitPlan(t, []commitPlanRow{{path: "one.md", content: "# One\n\nBody\n", action: importer.ActionCreate}})
	mock := NewMockClient()
	configureCreateCommit(mock, true)
	result := SetTestMode(mock)
	defer ResetTestMode()
	SetTestConfig("token", "https://api.example.com")
	stderr := setImportCommandIO(t, "no\n", true, false)
	resetImportCommitFlags(t)

	RunTestCommand(func() { importCommitCmd.Run(importCommitCmd, []string{planPath}) })

	if result.Response != nil || result.ExitCode != 0 {
		t.Fatalf("decline must emit no envelope and exit zero: %#v", result)
	}
	if len(mock.PostCalls) != 0 {
		t.Fatalf("decline wrote to the API: %#v", mock.PostCalls)
	}
	if strings.Count(stderr.String(), "? [y/N]") != 1 {
		t.Fatalf("expected exactly one prompt: %q", stderr.String())
	}
	if !strings.HasSuffix(stderr.String(), importCancelledNotice+"\n") {
		t.Fatalf("missing cancellation notice: %q", stderr.String())
	}
}

func TestImportCommitAcceptsCaseInsensitiveYes(t *testing.T) {
	for _, answer := range []string{"y\n", "Y\n", "yes\n", "YeS\n"} {
		t.Run(strings.TrimSpace(answer), func(t *testing.T) {
			planPath := writeCommitPlan(t, []commitPlanRow{{path: "one.md", content: "# One\n\nBody\n", action: importer.ActionCreate}})
			mock := NewMockClient()
			configureCreateCommit(mock, true)
			result := SetTestMode(mock)
			defer ResetTestMode()
			SetTestConfig("token", "https://api.example.com")
			setImportCommandIO(t, answer, true, false)
			resetImportCommitFlags(t)

			RunTestCommand(func() { importCommitCmd.Run(importCommitCmd, []string{planPath}) })

			if result.Response == nil || !result.Response.Success || len(mock.PostCalls) != 1 {
				t.Fatalf("answer %q did not execute: result=%#v posts=%#v", answer, result, mock.PostCalls)
			}
		})
	}
}

func TestImportCommitZeroWritesSkipsAuthNetworkPromptAndEnvelope(t *testing.T) {
	planPath := writeCommitPlan(t, []commitPlanRow{{path: "one.md", content: "# One\n\nBody\n", action: importer.ActionSkip}})
	for _, yes := range []bool{false, true} {
		t.Run(fmt.Sprintf("yes=%t", yes), func(t *testing.T) {
			mock := NewMockClient()
			result := SetTestMode(mock)
			defer ResetTestMode()
			stderr := setImportCommandIO(t, "", false, false)
			resetImportCommitFlags(t)
			importCommitYes = yes

			RunTestCommand(func() { importCommitCmd.Run(importCommitCmd, []string{planPath}) })

			if result.Response != nil || result.ExitCode != 0 {
				t.Fatalf("zero-write plan must emit no envelope: %#v", result)
			}
			if stderr.String() != importNoWritesNotice+"\n" {
				t.Fatalf("stderr = %q", stderr.String())
			}
			if len(mock.GetCalls) != 0 || len(mock.PostCalls) != 0 {
				t.Fatalf("zero-write plan touched API: gets=%#v posts=%#v", mock.GetCalls, mock.PostCalls)
			}
		})
	}
}

func TestImportCommitNonInteractiveRequiresYesBeforeAuthOrNetwork(t *testing.T) {
	planPath := writeCommitPlan(t, []commitPlanRow{{path: "one.md", content: "# One\n\nBody\n", action: importer.ActionCreate}})
	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()
	stderr := setImportCommandIO(t, "", false, false)
	resetImportCommitFlags(t)

	RunTestCommand(func() { importCommitCmd.Run(importCommitCmd, []string{planPath}) })

	if result.ExitCode != clierrors.ExitInvalidArgs || result.Response == nil || result.Response.Error == nil {
		t.Fatalf("expected invalid arguments: %#v", result)
	}
	if !strings.Contains(result.Response.Error.Message, "--yes") {
		t.Fatalf("error must name --yes: %#v", result.Response.Error)
	}
	if len(mock.GetCalls) != 0 || len(mock.PostCalls) != 0 || stderr.Len() != 0 {
		t.Fatalf("non-interactive rejection touched I/O: gets=%#v posts=%#v stderr=%q", mock.GetCalls, mock.PostCalls, stderr.String())
	}
}

func TestImportCommitNonInteractiveYesExecutesWithoutPrompt(t *testing.T) {
	planPath := writeCommitPlan(t, []commitPlanRow{{path: "one.md", content: "# One\n\nBody\n", action: importer.ActionCreate}})
	mock := NewMockClient()
	configureCreateCommit(mock, true)
	result := SetTestMode(mock)
	defer ResetTestMode()
	SetTestConfig("token", "https://api.example.com")
	stderr := setImportCommandIO(t, "", false, false)
	resetImportCommitFlags(t)
	importCommitYes = true

	RunTestCommand(func() { importCommitCmd.Run(importCommitCmd, []string{planPath}) })

	if result.ExitCode != 0 || result.Response == nil || !result.Response.Success {
		t.Fatalf("commit failed: %#v", result)
	}
	if stderr.Len() != 0 {
		t.Fatalf("non-TTY success wrote human guidance: %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "? [y/N]") {
		t.Fatalf("--yes prompted: %q", stderr.String())
	}
	assertCreateCommitEnvelope(t, result.Response, "Roasting notes", "https://app.example.com/workspaces/83")
}

func TestImportCommitWorkspaceLookupFailureIsBestEffort(t *testing.T) {
	planPath := writeCommitPlan(t, []commitPlanRow{{path: "one.md", content: "# One\n\nBody\n", action: importer.ActionCreate}})
	mock := NewMockClient()
	configureCreateCommit(mock, false)
	result := SetTestMode(mock)
	defer ResetTestMode()
	SetTestConfig("token", "https://api.example.com")
	stderr := setImportCommandIO(t, "", false, false)
	resetImportCommitFlags(t)
	importCommitYes = true

	RunTestCommand(func() { importCommitCmd.Run(importCommitCmd, []string{planPath}) })

	if result.ExitCode != 0 || result.Response == nil || !result.Response.Success {
		t.Fatalf("commit failed after lookup failure: %#v", result)
	}
	if got := stderr.String(); got != "Could not fetch the workspace name; continuing with workspace 83.\n" {
		t.Fatalf("unexpected warning: %q", got)
	}
	if result.Response.Summary != "Created 1 memory in workspace 83" || result.Response.Location != "" {
		t.Fatalf("lookup failure leaked metadata: %#v", result.Response)
	}
	if len(mock.PostCalls) != 1 {
		t.Fatalf("commit did not reach importer: %#v", mock.PostCalls)
	}
}

func TestImportCommitSuccessTTYBlockMatchesEnvelopeGuidance(t *testing.T) {
	planPath := writeCommitPlan(t, []commitPlanRow{{path: "one.md", content: "# One\n\nBody\n", action: importer.ActionCreate}})
	mock := NewMockClient()
	configureCreateCommit(mock, true)
	result := SetTestMode(mock)
	defer ResetTestMode()
	SetTestConfig("token", "https://api.example.com")
	stderr := setImportCommandIO(t, "", false, true)
	resetImportCommitFlags(t)
	importCommitYes = true

	RunTestCommand(func() { importCommitCmd.Run(importCommitCmd, []string{planPath}) })

	want := `Created 1 memory in workspace 83 (Roasting notes)

Workspace: https://app.example.com/workspaces/83

Next actions:
  recuerd0 workspace context 83 --pretty
  I just imported notes into recuerd0 workspace 83. Do the after-import pass.
`
	if stderr.String() != want {
		t.Fatalf("TTY guidance:\ngot:\n%s\nwant:\n%s", stderr.String(), want)
	}
	assertCreateCommitEnvelope(t, result.Response, "Roasting notes", "https://app.example.com/workspaces/83")
}

func TestImportCommitFailureRetainsPartialSummaryAndTypedError(t *testing.T) {
	planPath := writeCommitPlan(t, []commitPlanRow{{path: "one.md", content: "# One\n\nBody\n", action: importer.ActionCreate}})
	mock := NewMockClient()
	mock.GetResponses = []*client.APIResponse{{
		StatusCode: 200,
		Data:       map[string]interface{}{"id": "83"},
	}}
	mock.PostError = clierrors.NewValidationError("write rejected")
	result := SetTestMode(mock)
	defer ResetTestMode()
	SetTestConfig("token", "https://api.example.com")
	setImportCommandIO(t, "", false, false)
	resetImportCommitFlags(t)
	importCommitYes = true

	RunTestCommand(func() { importCommitCmd.Run(importCommitCmd, []string{planPath}) })

	if result.ExitCode != clierrors.ExitValidation || result.Response == nil || result.Response.Success {
		t.Fatalf("expected validation failure: %#v", result)
	}
	if result.Response.Error == nil || result.Response.Error.Code != clierrors.CodeValidation {
		t.Fatalf("typed error lost: %#v", result.Response.Error)
	}
	summary, ok := result.Response.Data.(importer.CommitSummary)
	if !ok || summary.Aborted == nil || !strings.Contains(summary.Aborted.Reason, "write rejected") {
		t.Fatalf("partial summary lost: %#v", result.Response.Data)
	}
}

func runImportProposeGuidanceTest(t *testing.T, source, planPath string, tty bool) (*response.Response, string) {
	t.Helper()

	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: []interface{}{}}
	result := SetTestMode(mock)
	defer ResetTestMode()

	stderr := setImportCommandIO(t, "", false, tty)
	importProposeWorkspace = "8"
	importProposePlan = planPath
	importProposeLedger = ""
	importProposeAdapter = ""
	importProposeFresh = false
	t.Cleanup(func() {
		importProposeWorkspace = ""
		importProposePlan = "import.plan.yaml"
		importProposeLedger = ""
		importProposeAdapter = ""
		importProposeFresh = false
	})

	SetTestConfig("token", "https://api.example.com")
	RunTestCommand(func() { importProposeCmd.Run(importProposeCmd, []string{source}) })
	if result.ExitCode != 0 || result.Response == nil || !result.Response.Success {
		t.Fatalf("propose failed: %#v", result)
	}
	return result.Response, stderr.String()
}

func normalizedResponseJSON(t *testing.T, resp *response.Response, pretty bool) []byte {
	t.Helper()

	resp.Meta["timestamp"] = "test"
	response.SetPrettyPrint(pretty)
	defer response.SetPrettyPrint(false)
	data, err := resp.JSON()
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n')
}

type commitPlanRow struct {
	path     string
	content  string
	action   string
	targetID int64
}

func writeCommitPlan(t *testing.T, rows []commitPlanRow) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if err := os.WriteFile(filepath.Join(source, row.path), []byte(row.content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	planPath := filepath.Join(root, "import.plan.yaml")
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: []interface{}{}}
	plan, _, err := importer.Propose(mock, importer.ProposeOptions{
		SourcePath: source,
		PlanPath:   planPath,
		Workspace:  83,
	})
	if err != nil {
		t.Fatal(err)
	}
	config := make(map[string]commitPlanRow, len(rows))
	for _, row := range rows {
		config[row.path] = row
	}
	for i := range plan.Manifest {
		row := config[plan.Manifest[i].Path]
		plan.Manifest[i].Action = row.action
		plan.Manifest[i].TargetMemoryID = row.targetID
		for j := range plan.Exceptions {
			if plan.Exceptions[j].Path == row.path {
				plan.Exceptions[j].Resolution = row.action
			}
		}
	}
	if err := importer.SavePlanAtomic(planPath, plan); err != nil {
		t.Fatal(err)
	}
	return planPath
}

func configureCreateCommit(mock *MockClient, workspaceFound bool) {
	workspace := map[string]interface{}{"id": "83"}
	if workspaceFound {
		workspace["name"] = "Roasting notes"
		workspace["url"] = "https://app.example.com/workspaces/83"
	}
	created := memoryResponse(101, "One", "# One\n\nBody\n", 1, "import:obsidian_markdown")
	mock.GetResponses = []*client.APIResponse{
		{StatusCode: 200, Data: workspace},
		{StatusCode: 200, Data: created},
	}
	mock.PostResponse = &client.APIResponse{StatusCode: 201, Data: created}
	mock.GetCalls = nil
	mock.PostCalls = nil
}

func memoryResponse(id int64, title, body string, version int, source string) map[string]interface{} {
	return map[string]interface{}{
		"id":       id,
		"title":    title,
		"version":  version,
		"source":   source,
		"tags":     []string{},
		"category": "general",
		"content":  map[string]interface{}{"body": body},
	}
}

func setImportCommandIO(t *testing.T, input string, inputTTY, stderrTTY bool) *bytes.Buffer {
	t.Helper()
	stderr := &bytes.Buffer{}
	oldInput := importCommitInput
	oldInputTTY := importCommitInputIsTTY
	oldOutput := importGuidanceOutput
	oldStderrTTY := importGuidanceIsTTY
	importCommitInput = strings.NewReader(input)
	importCommitInputIsTTY = func() bool { return inputTTY }
	importGuidanceOutput = stderr
	importGuidanceIsTTY = func() bool { return stderrTTY }
	t.Cleanup(func() {
		importCommitInput = oldInput
		importCommitInputIsTTY = oldInputTTY
		importGuidanceOutput = oldOutput
		importGuidanceIsTTY = oldStderrTTY
	})
	return stderr
}

func resetImportCommitFlags(t *testing.T) {
	t.Helper()
	importCommitYes = false
	importCommitLedger = ""
	t.Cleanup(func() {
		importCommitYes = false
		importCommitLedger = ""
	})
}

func assertCreateCommitEnvelope(t *testing.T, envelope *response.Response, name, location string) {
	t.Helper()
	if envelope == nil || !envelope.Success {
		t.Fatalf("missing success envelope: %#v", envelope)
	}
	if _, ok := envelope.Data.(importer.CommitSummary); !ok {
		t.Fatalf("commit data changed type: %T", envelope.Data)
	}
	wantSummary := "Created 1 memory in workspace 83"
	if name != "" {
		wantSummary += " (" + name + ")"
	}
	if envelope.Summary != wantSummary || envelope.Location != location {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
	if len(envelope.Breadcrumbs) != 2 ||
		envelope.Breadcrumbs[0].Action != "workspace-context" ||
		envelope.Breadcrumbs[1].Action != "after-import-pass" ||
		!strings.Contains(envelope.Breadcrumbs[1].Description, "Import leaves memories unstructured") {
		t.Fatalf("unexpected breadcrumbs: %#v", envelope.Breadcrumbs)
	}
}
