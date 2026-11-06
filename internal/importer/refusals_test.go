package importer

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
	clierrors "github.com/maquina/recuerd0-cli/internal/errors"
)

// refusingAPI refuses what a test names, so a refusal propose can't predict, such
// as a rule newer than this CLI, still reaches commit.
type refusingAPI struct {
	*testAPI
	refuseTitle string
	err         error
}

func (api *refusingAPI) Post(path string, body interface{}) (*client.APIResponse, error) {
	if wrapper, ok := body.(map[string]interface{}); ok {
		if memory, ok := wrapper["memory"].(map[string]interface{}); ok && memory["title"] == api.refuseTitle {
			return nil, api.err
		}
	}
	return api.testAPI.Post(path, body)
}

func refusedVault(t *testing.T) (*refusingAPI, ProposeOptions, string) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "a-bad.md"), "# Bad\n\nBody.\n")
	writeTestFile(t, filepath.Join(source, "b-good.md"), "# Good\n\nBody.\n")
	api := &refusingAPI{testAPI: newTestAPI(), refuseTitle: "Bad", err: clierrors.NewValidationError("a rule this CLI doesn't know")}
	options := ProposeOptions{SourcePath: source, PlanPath: filepath.Join(root, "import.plan.yaml"), Workspace: 3}
	if _, _, err := Propose(api, options); err != nil {
		t.Fatal(err)
	}
	summary, err := Commit(api, CommitOptions{PlanPath: options.PlanPath})
	if err == nil || summary.Aborted == nil || summary.Aborted.Path != "a-bad.md" {
		t.Fatalf("expected a stop at the refused row: %#v %v", summary, err)
	}
	if !strings.Contains(summary.Aborted.Next, "skip") || summary.Plan.Complete {
		t.Fatalf("a stop should say what to do and not read as complete: %#v", summary)
	}
	return api, options, source
}

func TestAFileFixedAfterARefusalImportsOnTheSameLedger(t *testing.T) {
	api, options, source := refusedVault(t)
	writeTestFile(t, filepath.Join(source, "a-bad.md"), "# Fixed\n\nBody.\n")

	plan, digest, err := Propose(api, options)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Manifest[0].Action != ActionCreate || len(digest.LeftOut) != 0 || len(plan.Exceptions) != 0 {
		t.Fatalf("nothing landed, so the fixed file is a fresh create: %#v", plan)
	}
	summary, err := Commit(api, CommitOptions{PlanPath: options.PlanPath})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Ops.Created != 2 || !summary.Plan.Complete || summary.Plan.RowsCommitted != 2 {
		t.Fatalf("both files should land: %#v", summary)
	}

	// The finished import is an ordinary one from here on.
	plan, _, err = Propose(api, options)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range plan.Manifest {
		if row.Action != ActionSkip {
			t.Fatalf("already imported: %#v", row)
		}
	}
}

func TestARefusedFileCanRetryUnchanged(t *testing.T) {
	api, options, _ := refusedVault(t)
	if _, err := Commit(api, CommitOptions{PlanPath: options.PlanPath}); err == nil {
		t.Fatal("still refused, so it should still stop")
	}
	api.refuseTitle = ""
	summary, err := Commit(api, CommitOptions{PlanPath: options.PlanPath})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Ops.Created != 2 || !summary.Plan.Complete {
		t.Fatalf("once the server accepts it, the plan finishes: %#v", summary)
	}
}

func TestARefusedRowCanBeSkippedAndTheRestFinishes(t *testing.T) {
	api, options, _ := refusedVault(t)
	plan, err := LoadPlan(options.PlanPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := range plan.Manifest {
		if plan.Manifest[i].Path == "a-bad.md" {
			plan.Manifest[i].Action = ActionSkip
		}
	}
	if err := SavePlanAtomic(options.PlanPath, plan); err != nil {
		t.Fatal(err)
	}
	summary, err := Commit(api, CommitOptions{PlanPath: options.PlanPath})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Ops.Created != 1 || summary.Plan.RowsReviewSkip != 1 || summary.Plan.RowsLeftOut != 0 || !summary.Plan.Complete {
		t.Fatalf("a chosen skip is not a row left out: %#v", summary)
	}
}

func TestProposeFlagsARefusedFileItNowKnowsAbout(t *testing.T) {
	api, options, source := refusedVault(t)
	writeTestFile(t, filepath.Join(source, "a-bad.md"), manyTags(51)+"# Bad\n\nBody.\n")
	plan, _, err := Propose(api, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := exceptionsFor(plan, "a-bad.md")["unimportable"]; !ok {
		t.Fatalf("expected a skip: %#v", plan.Exceptions)
	}
	if _, err := Commit(api, CommitOptions{PlanPath: options.PlanPath}); err != nil {
		t.Fatal(err)
	}
}

func TestALostResponseStaysUnsettledUntilCommitChecks(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "note.md"), "# Note\n\nBody.\n")
	base := newTestAPI()
	planPath := filepath.Join(root, "import.plan.yaml")
	options := ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 3}
	plan, _, err := Propose(base, options)
	if err != nil {
		t.Fatal(err)
	}

	// All an older CLI left behind for a refusal, and all a crash leaves.
	row := plan.Manifest[0]
	ledgerPath := filepath.Join(root, "import.ledger.jsonl")
	if err := AppendLedgerRecord(ledgerPath, newIntent(row.Path, 3, ActionCreate, 1, 1, row.SourceHash, row.ContentHash, 1, 0, 0)); err != nil {
		t.Fatal(err)
	}

	skipped := *plan
	skipped.Manifest = append([]PlanRow(nil), plan.Manifest...)
	skipped.Manifest[0].Action = ActionSkip
	if err := SavePlanAtomic(planPath, &skipped); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Review(CommitOptions{PlanPath: planPath}); err == nil || !strings.Contains(err.Error(), "run commit again to settle it") {
		t.Fatalf("the write may have landed, so a skip can't be taken yet: %v", err)
	}

	writeTestFile(t, filepath.Join(source, "note.md"), "# Changed\n\nBody.\n")
	changed, digest, err := Propose(base, options)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Manifest[0].Action != ActionSkip || len(digest.LeftOut) != 1 {
		t.Fatalf("an unsettled write can't be replaced: %#v %#v", changed.Manifest[0], digest)
	}
	summary, err := Commit(base, CommitOptions{PlanPath: planPath})
	if err == nil || !strings.Contains(err.Error(), "note.md") || summary.Plan.Complete {
		t.Fatalf("a row left out can't read as success: %#v %v", summary, err)
	}
	if len(base.postCalls) != 0 {
		t.Fatalf("nothing should be written: %#v", base.postCalls)
	}
}

func TestAReadBackFailureIsNotRecordedAsARefusal(t *testing.T) {
	base := newTestAPI()
	planPath := proposeSingleMarkdown(t, base, "# One\n\nBody\n")
	api := &hookedAPI{base: base}
	api.getHook = func(api *hookedAPI, path string) (*client.APIResponse, error) {
		return nil, clierrors.NewNotFoundError("gone")
	}
	if _, err := Commit(api, CommitOptions{PlanPath: planPath}); err == nil {
		t.Fatal("expected a stop")
	}
	ledger, err := LoadLedger(filepath.Join(filepath.Dir(planPath), "import.ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range ledger.Records {
		if record.Kind == "refused" {
			t.Fatalf("the write succeeded, and a 404 reading it back doesn't mean otherwise: %#v", ledger.Records)
		}
	}
}
