package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
	"github.com/maquina/recuerd0-cli/internal/response"
)

const importHelpSentence = "Import is propose → review → commit: `propose` writes a reviewable `import.plan.yaml` and never touches the server; `commit` executes the plan you approved."

func TestImportHelpContainsCanonicalLoop(t *testing.T) {
	if !strings.Contains(importCmd.Long, importHelpSentence) {
		t.Fatalf("import help changed:\n%s", importCmd.Long)
	}
}

func TestImportProposeAndDryRunShareDigestShape(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "one.md"), []byte("# One\n\nBody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(root, "import.plan.yaml")
	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: []interface{}{}}
	result := SetTestMode(mock)
	SetTestConfig("token", "https://api.example.com")
	defer ResetTestMode()
	defer func() {
		importProposeWorkspace = ""
		importProposePlan = "import.plan.yaml"
		importProposeLedger = ""
		importProposeAdapter = ""
		importProposeFresh = false
		importCommitYes = false
		importCommitLedger = ""
		importCommitDryRun = false
	}()

	importProposeWorkspace = "8"
	importProposePlan = planPath
	RunTestCommand(func() { importProposeCmd.Run(importProposeCmd, []string{source}) })
	if result.ExitCode != 0 || result.Response == nil || !result.Response.Success {
		t.Fatalf("propose failed: %#v", result)
	}
	proposeDigest := result.Response.Data

	result.Response = nil
	result.ExitCode = 0
	importCommitYes = true
	importCommitDryRun = true
	RunTestCommand(func() { importCommitCmd.Run(importCommitCmd, []string{planPath}) })
	if result.ExitCode != 1 || result.Response == nil || !result.Response.Success {
		t.Fatalf("dry-run should be success-shaped exit 1: %#v", result)
	}
	if len(mock.PostCalls) != 0 {
		t.Fatalf("--dry-run must win over --yes; got POSTs %#v", mock.PostCalls)
	}
	if !sameDigestShape(proposeDigest, result.Response.Data) {
		t.Fatalf("propose and dry-run digest shapes differ: %#v vs %#v", proposeDigest, result.Response.Data)
	}
}

func TestImportProposeGuidanceUsesDisplayPathOnlyForTTYStderr(t *testing.T) {
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

  recuerd0 import commit %s --dry-run    validate without writing
  recuerd0 import commit %s              execute after review
`, displayPath, displayPath, displayPath)
				if ttyStderr != wantGuidance {
					t.Fatalf("unexpected TTY guidance:\ngot:\n%s\nwant:\n%s", ttyStderr, wantGuidance)
				}
				if strings.Count(ttyStderr, displayPath) != 3 {
					t.Fatalf("display path must appear three times, got %q", ttyStderr)
				}
			}
		})
	}
}

func runImportProposeGuidanceTest(t *testing.T, source, planPath string, tty bool) (*response.Response, string) {
	t.Helper()

	mock := NewMockClient()
	mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: []interface{}{}}
	result := SetTestMode(mock)
	defer ResetTestMode()

	var stderr bytes.Buffer
	oldOutput := importGuidanceOutput
	oldIsTTY := importGuidanceIsTTY
	importGuidanceOutput = &stderr
	importGuidanceIsTTY = func() bool { return tty }
	defer func() {
		importGuidanceOutput = oldOutput
		importGuidanceIsTTY = oldIsTTY
	}()

	importProposeWorkspace = "8"
	importProposePlan = planPath
	importProposeLedger = ""
	importProposeAdapter = ""
	importProposeFresh = false
	defer func() {
		importProposeWorkspace = ""
		importProposePlan = "import.plan.yaml"
		importProposeLedger = ""
		importProposeAdapter = ""
		importProposeFresh = false
	}()

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

func sameDigestShape(first, second interface{}) bool {
	firstMap := structFieldNames(first)
	secondMap := structFieldNames(second)
	if len(firstMap) != len(secondMap) {
		return false
	}
	for key := range firstMap {
		if !secondMap[key] {
			return false
		}
	}
	return true
}

func structFieldNames(value interface{}) map[string]bool {
	// Both values are importer.Digest structs in command responses. JSON field
	// names are stable by contract, and a marshal round-trip avoids importing
	// importer just for reflection on its Go field names.
	data, _ := json.Marshal(value)
	result := make(map[string]interface{})
	_ = json.Unmarshal(data, &result)
	keys := make(map[string]bool, len(result))
	for key := range result {
		keys[key] = true
	}
	return keys
}
