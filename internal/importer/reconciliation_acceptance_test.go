package importer

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
	clierrors "github.com/maquina/recuerd0-cli/internal/errors"
)

type hookedAPI struct {
	base      *testAPI
	postCount int
	postHook  func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error)
}

func (api *hookedAPI) Get(path string) (*client.APIResponse, error) {
	return api.base.Get(path)
}

func (api *hookedAPI) GetWithPagination(path string) (*client.APIResponse, error) {
	return api.base.GetWithPagination(path)
}

func (api *hookedAPI) Post(path string, body interface{}) (*client.APIResponse, error) {
	api.postCount++
	if api.postHook != nil {
		return api.postHook(api, path, body)
	}
	return api.base.Post(path, body)
}

func (api *hookedAPI) Patch(path string, body interface{}) (*client.APIResponse, error) {
	return api.base.Patch(path, body)
}

func (api *hookedAPI) Delete(path string) (*client.APIResponse, error) {
	return api.base.Delete(path)
}

func TestCreateReconciliationZeroOneManyCandidates(t *testing.T) {
	t.Run("zero candidates retries once", func(t *testing.T) {
		base := newTestAPI()
		planPath := proposeSingleMarkdown(t, base, "# One\n\nBody\n")
		api := &hookedAPI{base: base}
		api.postHook = func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error) {
			if api.postCount == 1 {
				base.postCalls = append(base.postCalls, testCall{path: path, body: body})
				return nil, clierrors.NewNetworkError("lost before write")
			}
			return base.Post(path, body)
		}

		summary, err := Commit(api, CommitOptions{PlanPath: planPath})
		if err != nil {
			t.Fatal(err)
		}
		if api.postCount != 2 || summary.Ops.Created != 1 ||
			summary.Ops.Reconciled != 0 || !summary.Plan.Complete {
			t.Fatalf("zero-candidate retry summary mismatch: %#v posts=%d", summary, api.postCount)
		}
		assertPass1IntentsEqualPosts(t, filepath.Join(filepath.Dir(planPath), "import.ledger.jsonl"), 2)
	})

	t.Run("one exact candidate reconciles without retry", func(t *testing.T) {
		base := newTestAPI()
		planPath := proposeSingleMarkdown(t, base, "# One\n\nBody\n")
		api := &hookedAPI{base: base}
		api.postHook = func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error) {
			response, err := base.Post(path, body)
			if err != nil {
				return response, err
			}
			return nil, clierrors.NewNetworkError("response lost after write")
		}

		summary, err := Commit(api, CommitOptions{PlanPath: planPath})
		if err != nil {
			t.Fatal(err)
		}
		if api.postCount != 1 || summary.Ops.Created != 1 ||
			summary.Ops.Reconciled != 1 || !summary.Plan.Complete {
			t.Fatalf("one-candidate reconciliation mismatch: %#v posts=%d", summary, api.postCount)
		}
		ledger, err := LoadLedger(filepath.Join(filepath.Dir(planPath), "import.ledger.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if len(ledger.Records) != 2 || !ledger.Records[1].Reconciled {
			t.Fatalf("expected reconciled committed record: %#v", ledger.Records)
		}
	})

	t.Run("many exact candidates abort ambiguously", func(t *testing.T) {
		base := newTestAPI()
		planPath := proposeSingleMarkdown(t, base, "# One\n\nBody\n")
		api := &hookedAPI{base: base}
		api.postHook = func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error) {
			response, err := base.Post(path, body)
			if err != nil {
				return response, err
			}
			first := base.memories[base.nextID]
			base.nextID++
			second := first
			second.ID = base.nextID
			base.memories[second.ID] = second
			return nil, clierrors.NewNetworkError("response lost after duplicate writes")
		}

		summary, err := Commit(api, CommitOptions{PlanPath: planPath})
		if err == nil || summary.Aborted == nil ||
			!strings.Contains(summary.Aborted.Reason, "multiple exact create candidates") {
			t.Fatalf("many candidates must abort ambiguously: summary=%#v err=%v", summary, err)
		}
		if api.postCount != 1 || summary.Ops.Created != 0 ||
			summary.Plan.RowsRemaining != 1 || summary.Plan.Complete {
			t.Fatalf("ambiguous create summary is not truthful: %#v posts=%d", summary, api.postCount)
		}
	})
}

func TestVersionWriteLostResponseReconcilesFixedIdentity(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "one.md"), "# One\n\nNew body\n")
	planPath := filepath.Join(root, "import.plan.yaml")
	base := newTestAPI()
	base.memories[50] = memoryRecord{
		ID: 50, Title: "One", Body: "Old body", Tags: []string{"old"},
		Category: "general", Source: "manual", Version: 7,
	}
	if _, _, err := Propose(base, ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 1}); err != nil {
		t.Fatal(err)
	}
	approveFirstVersion(t, planPath)

	api := &hookedAPI{base: base}
	api.postHook = func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error) {
		response, err := base.Post(path, body)
		if err != nil {
			return response, err
		}
		return nil, clierrors.NewNetworkError("version response lost after write")
	}
	summary, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if api.postCount != 1 || summary.Ops.Versioned != 1 ||
		summary.Ops.Reconciled != 1 || base.memories[50].Version != 8 ||
		!summary.Plan.Complete {
		t.Fatalf("lost version response was not reconciled: %#v memory=%#v", summary, base.memories[50])
	}
	ledger, err := LoadLedger(filepath.Join(root, "import.ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range ledger.Records {
		if record.MemoryID != 50 || record.ChainBase != 7 {
			t.Fatalf("version reconciliation changed fixed identity/base: %#v", record)
		}
	}
}

func TestDanglingLandedCreateIntentReconcilesOnResume(t *testing.T) {
	base := newTestAPI()
	planPath := proposeSingleMarkdown(t, base, "# One\n\nBody\n")
	plan, err := LoadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	row := plan.Manifest[0]
	ledgerPath := filepath.Join(filepath.Dir(planPath), "import.ledger.jsonl")
	intent := newIntent(row.Path, plan.Workspace, ActionCreate, 1, 1,
		row.SourceHash, row.ContentHash, 1, 0, 0)
	if err := AppendLedgerRecord(ledgerPath, intent); err != nil {
		t.Fatal(err)
	}
	base.memories[77] = memoryRecord{
		ID: 77, Title: row.Title, Body: "# One\n\nBody\n", Tags: row.Tags,
		Category: row.Category, Source: "import:" + plan.Adapter, Version: 1,
	}

	summary, err := Commit(base, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(base.postCalls) != 0 || summary.Ops.Created != 1 ||
		summary.Ops.Reconciled != 1 || summary.Rows.CompletedNow != 1 ||
		!summary.Plan.Complete {
		t.Fatalf("dangling landed intent did not reconcile: %#v posts=%#v", summary, base.postCalls)
	}
	ledger, err := LoadLedger(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Paths[row.Path].MemoryID != 77 ||
		ledger.Paths[row.Path].Revisions[0].CommittedPrefix != 1 {
		t.Fatalf("resume did not persist reconciled identity: %#v", ledger.Paths[row.Path])
	}
}

func TestPass1ValidationErrorDoesNotRetry(t *testing.T) {
	base := newTestAPI()
	planPath := proposeSingleMarkdown(t, base, "# One\n\nBody\n")
	api := &hookedAPI{base: base}
	api.postHook = func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error) {
		base.postCalls = append(base.postCalls, testCall{path: path, body: body})
		return nil, clierrors.NewValidationError("rejected")
	}

	summary, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err == nil || api.postCount != 1 || summary.Aborted == nil ||
		summary.Aborted.Ordinal != 1 || summary.Ops.Created != 0 ||
		summary.Plan.RowsRemaining != 1 {
		t.Fatalf("pass-1 422 must abort without retry: summary=%#v posts=%d err=%v", summary, api.postCount, err)
	}
	ledger, err := LoadLedger(filepath.Join(filepath.Dir(planPath), "import.ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Records) != 1 || ledger.Records[0].Kind != "intent" {
		t.Fatalf("422 ledger should contain only the attempted intent: %#v", ledger.Records)
	}
}

func TestReadBackMismatchAbortsWithoutCommittedRecord(t *testing.T) {
	base := newTestAPI()
	planPath := proposeSingleMarkdown(t, base, "# One\n\nBody\n")
	api := &hookedAPI{base: base}
	api.postHook = func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error) {
		response, err := base.Post(path, body)
		if err == nil {
			memory := base.memories[base.nextID]
			memory.Body = "server-mutated body"
			base.memories[memory.ID] = memory
		}
		return response, err
	}

	summary, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err == nil || summary.Aborted == nil ||
		!strings.Contains(summary.Aborted.Reason, "read-back body mismatch") ||
		api.postCount != 1 || summary.Ops.Created != 0 {
		t.Fatalf("read-back mismatch must abort: summary=%#v err=%v", summary, err)
	}
	ledger, err := LoadLedger(filepath.Join(filepath.Dir(planPath), "import.ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Records) != 1 || ledger.Records[0].Kind != "intent" {
		t.Fatalf("read-back mismatch must not append committed: %#v", ledger.Records)
	}
}

func TestPartialAbortAndResumeUseInvocationOnlySummaryCounters(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "a.md"), "# A\n\nFirst\n")
	writeTestFile(t, filepath.Join(source, "b.md"), "# B\n\nSecond\n")
	planPath := filepath.Join(root, "import.plan.yaml")
	base := newTestAPI()
	if _, _, err := Propose(base, ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 1}); err != nil {
		t.Fatal(err)
	}
	api := &hookedAPI{base: base}
	api.postHook = func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error) {
		memory := body.(map[string]interface{})["memory"].(map[string]interface{})
		if memory["title"] == "B" {
			base.postCalls = append(base.postCalls, testCall{path: path, body: body})
			return nil, clierrors.NewValidationError("stop after first row")
		}
		return base.Post(path, body)
	}

	first, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err == nil || first.Aborted == nil || first.Aborted.Path != "b.md" ||
		first.Ops.Created != 1 || first.Rows.CompletedNow != 1 ||
		first.Rows.AlreadyCommitted != 0 || first.Plan.RowsCommitted != 1 ||
		first.Plan.RowsRemaining != 1 || first.Plan.Complete {
		t.Fatalf("partial abort summary mismatch: %#v err=%v", first, err)
	}

	second, err := Commit(base, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if second.Ops.Created != 1 || second.Rows.CompletedNow != 1 ||
		second.Rows.AlreadyCommitted != 1 || second.Plan.RowsCommitted != 2 ||
		second.Plan.RowsRemaining != 0 || !second.Plan.Complete {
		t.Fatalf("resume counters must be invocation-only: %#v", second)
	}
}

func proposeSingleMarkdown(t *testing.T, api client.API, content string) string {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "one.md"), content)
	planPath := filepath.Join(root, "import.plan.yaml")
	if _, _, err := Propose(api, ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 1}); err != nil {
		t.Fatal(err)
	}
	return planPath
}

func approveFirstVersion(t *testing.T, planPath string) {
	t.Helper()
	plan, err := LoadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	plan.Manifest[0].Action = ActionVersion
	for i := range plan.Exceptions {
		if plan.Exceptions[i].Path == plan.Manifest[0].Path {
			plan.Exceptions[i].Resolution = ActionVersion
		}
	}
	if err := SavePlanAtomic(planPath, plan); err != nil {
		t.Fatal(err)
	}
}

func assertPass1IntentsEqualPosts(t *testing.T, ledgerPath string, posts int) {
	t.Helper()
	ledger, err := LoadLedger(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	intents := 0
	for _, record := range ledger.Records {
		if record.Kind == "intent" {
			intents++
		}
	}
	if intents != posts {
		t.Fatalf("every pass-1 POST needs an intent: intents=%d posts=%d", intents, posts)
	}
}
