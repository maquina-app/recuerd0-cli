package importer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPlanValidationDanglingExceptionsAndTargetContradictions(t *testing.T) {
	t.Run("dangling exception reports its line", func(t *testing.T) {
		api := newTestAPI()
		planPath := proposeSingleMarkdown(t, api, "# One\n\nBody\n")
		plan, _ := LoadPlan(planPath)
		plan.Exceptions = append(plan.Exceptions, Exception{
			Path: "missing.md", Kind: "conflict", Resolution: ActionSkip,
		})
		writePlanWithoutValidation(t, planPath, plan)
		_, err := LoadPlan(planPath)
		if err == nil || !strings.Contains(err.Error(), "exception references missing manifest row") ||
			!strings.Contains(err.Error(), "line ") {
			t.Fatalf("expected line-numbered dangling exception error, got %v", err)
		}
	})

	t.Run("create cannot carry target_memory_id", func(t *testing.T) {
		api := newTestAPI()
		planPath := proposeSingleMarkdown(t, api, "# One\n\nBody\n")
		plan, _ := LoadPlan(planPath)
		plan.Manifest[0].TargetMemoryID = 99
		writePlanWithoutValidation(t, planPath, plan)
		_, err := LoadPlan(planPath)
		if err == nil || !strings.Contains(err.Error(), "target_memory_id is invalid with create") ||
			!strings.Contains(err.Error(), "line ") {
			t.Fatalf("expected line-numbered create target error, got %v", err)
		}
	})

	t.Run("version requires plan or ledger identity", func(t *testing.T) {
		api := newTestAPI()
		planPath := proposeSingleMarkdown(t, api, "# One\n\nBody\n")
		plan, _ := LoadPlan(planPath)
		plan.Manifest[0].Action = ActionVersion
		if err := SavePlanAtomic(planPath, plan); err != nil {
			t.Fatal(err)
		}
		_, _, _, err := Review(CommitOptions{PlanPath: planPath})
		if err == nil || !strings.Contains(err.Error(), "version requires target_memory_id or ledger identity") ||
			!strings.Contains(err.Error(), "line ") {
			t.Fatalf("expected line-numbered missing identity error, got %v", err)
		}
	})

	t.Run("plan target must agree with ledger identity", func(t *testing.T) {
		api := newTestAPI()
		planPath := proposeSingleMarkdown(t, api, "# One\n\nBody\n")
		plan, _ := LoadPlan(planPath)
		row := plan.Manifest[0]
		plan.Manifest[0].Action = ActionVersion
		plan.Manifest[0].TargetMemoryID = 8
		if err := SavePlanAtomic(planPath, plan); err != nil {
			t.Fatal(err)
		}
		ledgerPath := filepath.Join(filepath.Dir(planPath), "import.ledger.jsonl")
		intent := newIntent(row.Path, plan.Workspace, ActionCreate, 1, 1,
			row.SourceHash, row.ContentHash, 1, 0, 0)
		appendRecordForTest(t, ledgerPath, intent)
		appendRecordForTest(t, ledgerPath, newCommitted(intent, 9, false))
		_, _, _, err := Review(CommitOptions{PlanPath: planPath})
		if err == nil || !strings.Contains(err.Error(), "disagrees with ledger memory_id 9") ||
			!strings.Contains(err.Error(), "line ") {
			t.Fatalf("expected line-numbered target disagreement, got %v", err)
		}
	})
}

func TestLedgerCorruptionIdentityBaseAndCommittedPrefixValidation(t *testing.T) {
	sourceHash := sourceBodyHash("body")
	contentHash := CanonicalTupleHash("T", nil, "general", "body")

	t.Run("malformed JSON reports exact line", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ledger.jsonl")
		intent := newIntent("a.md", 1, ActionCreate, 1, 1, sourceHash, contentHash, 1, 0, 0)
		raw, _ := json.Marshal(intent)
		writeTestFile(t, path, string(raw)+"\n{\"kind\":\n")
		_, err := LoadLedger(path)
		if err == nil || !strings.Contains(err.Error(), "ledger line 2") {
			t.Fatalf("expected ledger line 2 corruption, got %v", err)
		}
	})

	t.Run("memory identity is immutable across revisions", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ledger.jsonl")
		first := newIntent("a.md", 1, ActionCreate, 1, 1, sourceHash, contentHash, 1, 0, 0)
		appendRecordForTest(t, path, first)
		appendRecordForTest(t, path, newCommitted(first, 9, false))
		secondHash := sourceBodyHash("changed")
		secondContent := CanonicalTupleHash("T", nil, "general", "changed")
		second := newIntent("a.md", 1, ActionVersion, 1, 1, secondHash, secondContent, 2, 1, 10)
		appendRecordForTest(t, path, second)
		_, err := LoadLedger(path)
		if err == nil || !strings.Contains(err.Error(), "line 3") ||
			!strings.Contains(err.Error(), "memory_id 10") {
			t.Fatalf("expected immutable memory identity error, got %v", err)
		}
	})

	t.Run("chain base is immutable across retry intents", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ledger.jsonl")
		first := newIntent("a.md", 1, ActionCreate, 1, 1, sourceHash, contentHash, 1, 0, 0)
		appendRecordForTest(t, path, first)
		second := newIntent("a.md", 1, ActionCreate, 1, 1, sourceHash, contentHash, 2, 1, 0)
		appendRecordForTest(t, path, second)
		_, err := LoadLedger(path)
		if err == nil || !strings.Contains(err.Error(), "line 2") ||
			!strings.Contains(err.Error(), "chain_base") {
			t.Fatalf("expected immutable chain base error, got %v", err)
		}
	})

	t.Run("committed prefix cannot contain a gap", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "ledger.jsonl")
		intent := newIntent("a.md", 1, ActionCreate, 2, 2, sourceHash, contentHash, 2, 0, 9)
		appendRecordForTest(t, path, intent)
		appendRecordForTest(t, path, newCommitted(intent, 9, false))
		_, err := LoadLedger(path)
		if err == nil || !strings.Contains(err.Error(), "gap before ordinal 2") {
			t.Fatalf("expected committed-prefix gap error, got %v", err)
		}
	})
}

func TestSavePlanAtomicIsDeterministicAndLeavesNoTemporaryFile(t *testing.T) {
	api := newTestAPI()
	planPath := proposeSingleMarkdown(t, api, "# One\n\nBody\n")
	plan, err := LoadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(planPath)
	if err := SavePlanAtomic(planPath, plan); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(planPath)
	if string(first) != string(second) {
		t.Fatal("atomic plan serialization is not deterministic")
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(planPath), "."+filepath.Base(planPath)+".tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("atomic save left temporary files: %#v", matches)
	}
}

func writePlanWithoutValidation(t *testing.T, path string, plan *Plan) {
	t.Helper()
	data, err := yaml.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
