package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
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
