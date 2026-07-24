package importer

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
	clierrors "github.com/maquina/recuerd0-cli/internal/errors"
)

func TestLinkFailureReconciliationAndRetryPolicy(t *testing.T) {
	t.Run("lost network response reconciles an existing pair", func(t *testing.T) {
		base, planPath := proposeLinkedMarkdown(t)
		api := &hookedAPI{base: base}
		linkAttempts := 0
		api.postHook = func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error) {
			if !strings.HasSuffix(path, "/links") {
				return base.Post(path, body)
			}
			linkAttempts++
			response, err := base.Post(path, body)
			if err != nil {
				return response, err
			}
			return nil, clierrors.NewNetworkError("link response lost after write")
		}

		summary, err := Commit(api, CommitOptions{PlanPath: planPath})
		if err != nil {
			t.Fatal(err)
		}
		if linkAttempts != 1 || summary.LinksEnsured.Existing != 1 ||
			summary.LinksEnsured.Created != 0 || len(summary.LinksFailed) != 0 ||
			!summary.Plan.Complete {
			t.Fatalf("lost link response was not reconciled: attempts=%d summary=%#v", linkAttempts, summary)
		}
	})

	t.Run("absent 422 retries once", func(t *testing.T) {
		base, planPath := proposeLinkedMarkdown(t)
		api := &hookedAPI{base: base}
		linkAttempts := 0
		api.postHook = func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error) {
			if !strings.HasSuffix(path, "/links") {
				return base.Post(path, body)
			}
			linkAttempts++
			if linkAttempts == 1 {
				return nil, clierrors.NewValidationError("temporarily absent")
			}
			return base.Post(path, body)
		}

		summary, err := Commit(api, CommitOptions{PlanPath: planPath})
		if err != nil {
			t.Fatal(err)
		}
		if linkAttempts != 2 || summary.LinksEnsured.Created != 1 ||
			len(summary.LinksFailed) != 0 || !summary.Plan.Complete {
			t.Fatalf("absent 422 did not retry once: attempts=%d summary=%#v", linkAttempts, summary)
		}
	})

	t.Run("absent 5xx retries once", func(t *testing.T) {
		base, planPath := proposeLinkedMarkdown(t)
		api := &hookedAPI{base: base}
		linkAttempts := 0
		api.postHook = func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error) {
			if !strings.HasSuffix(path, "/links") {
				return base.Post(path, body)
			}
			linkAttempts++
			if linkAttempts == 1 {
				return nil, clierrors.FromHTTPStatus(503, "temporarily unavailable")
			}
			return base.Post(path, body)
		}

		summary, err := Commit(api, CommitOptions{PlanPath: planPath})
		if err != nil {
			t.Fatal(err)
		}
		if linkAttempts != 2 || summary.LinksEnsured.Created != 1 ||
			len(summary.LinksFailed) != 0 || !summary.Plan.Complete {
			t.Fatalf("absent 5xx did not retry once: attempts=%d summary=%#v", linkAttempts, summary)
		}
	})

	t.Run("non-422 4xx does not retry and remains non-fatal", func(t *testing.T) {
		base, planPath := proposeLinkedMarkdown(t)
		api := &hookedAPI{base: base}
		linkAttempts := 0
		api.postHook = func(api *hookedAPI, path string, body interface{}) (*client.APIResponse, error) {
			if !strings.HasSuffix(path, "/links") {
				return base.Post(path, body)
			}
			linkAttempts++
			return nil, clierrors.FromHTTPStatus(400, "bad link request")
		}

		summary, err := Commit(api, CommitOptions{PlanPath: planPath})
		if err != nil {
			t.Fatalf("link failures must remain non-fatal: %v", err)
		}
		if linkAttempts != 1 || len(summary.LinksFailed) != 1 ||
			summary.LinksFailed[0].FromPath != "a.md" ||
			summary.LinksFailed[0].ToPath != "b.md" ||
			summary.Plan.Complete {
			t.Fatalf("non-422 4xx policy mismatch: attempts=%d summary=%#v", linkAttempts, summary)
		}
	})
}

func TestFreshExportRepeatedTuplesKeepFixedRootIdentityAndOrdinalVersions(t *testing.T) {
	root := t.TempDir()
	exportPath := filepath.Join(root, "export.json")
	writeTestFile(t, exportPath, `{
  "format": "recuerd0.workspace_export",
  "format_version": 1,
  "workspace": {"id": 22},
  "memories": [{
    "root_id": 7,
    "versions": [
      {"version": 1, "title": "Repeated", "body": "same", "tags": ["b", "a"], "category": "general"},
      {"version": 2, "title": "Repeated", "body": "same", "tags": ["b", "a"], "category": "general"}
    ]
  }]
}`)
	planPath := filepath.Join(root, "import.plan.yaml")
	api := newTestAPI()
	if _, _, err := Propose(api, ProposeOptions{
		SourcePath: exportPath, PlanPath: planPath, Workspace: 3,
		Adapter: AdapterWorkspaceExport,
	}); err != nil {
		t.Fatal(err)
	}

	summary, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Ops.Created != 1 || summary.Ops.Versioned != 1 ||
		summary.Ops.Reconciled != 0 || !summary.Plan.Complete {
		t.Fatalf("fresh repeated export chain summary mismatch: %#v", summary)
	}
	if got := api.memories[101]; got.ID != 101 || got.Version != 2 ||
		got.Title != "Repeated" || got.Body != "same" {
		t.Fatalf("fresh export chain did not land at fixed root version 2: %#v", got)
	}

	ledger, err := LoadLedger(filepath.Join(root, "import.ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Records) != 4 {
		t.Fatalf("expected two intent/committed pairs, got %#v", ledger.Records)
	}
	for i, record := range ledger.Records {
		wantOrdinal := i/2 + 1
		if record.Ordinal != wantOrdinal || record.ChainLen != 2 ||
			record.ChainBase != 0 || (record.MemoryID != 0 && record.MemoryID != 101) {
			t.Fatalf("export ledger identity/arithmetic mismatch at line %d: %#v", i+1, record)
		}
		if record.Kind == "committed" && record.Version != wantOrdinal {
			t.Fatalf("committed export version mismatch at line %d: %#v", i+1, record)
		}
	}
}

func TestChangedAndDifferentRevisionExportChainsRemainSkipped(t *testing.T) {
	t.Run("changed completed export is not delta merged", func(t *testing.T) {
		root := t.TempDir()
		exportPath := filepath.Join(root, "export.json")
		writeWorkspaceExportForTest(t, exportPath, "before", 1)
		planPath := filepath.Join(root, "import.plan.yaml")
		api := newTestAPI()
		options := ProposeOptions{
			SourcePath: exportPath, PlanPath: planPath, Workspace: 3,
			Adapter: AdapterWorkspaceExport,
		}
		if _, _, err := Propose(api, options); err != nil {
			t.Fatal(err)
		}
		if _, err := Commit(api, CommitOptions{PlanPath: planPath}); err != nil {
			t.Fatal(err)
		}

		writeWorkspaceExportForTest(t, exportPath, "after", 1)
		plan, _, err := Propose(api, options)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Manifest[0].Action != ActionSkip ||
			plan.Manifest[0].TargetMemoryID != 101 ||
			!hasException(plan.Exceptions, "conflict") ||
			!exceptionDetailContains(plan.Exceptions, "completed workspace export changed") {
			t.Fatalf("changed completed export was not conservatively skipped: %#v", plan)
		}
	})

	t.Run("different revision partial export cannot be approved for writing", func(t *testing.T) {
		root := t.TempDir()
		exportPath := filepath.Join(root, "export.json")
		writeWorkspaceExportForTest(t, exportPath, "before", 2)
		planPath := filepath.Join(root, "import.plan.yaml")
		api := newTestAPI()
		options := ProposeOptions{
			SourcePath: exportPath, PlanPath: planPath, Workspace: 3,
			Adapter: AdapterWorkspaceExport,
		}
		if _, _, err := Propose(api, options); err != nil {
			t.Fatal(err)
		}
		api.failVersion = 2
		api.failVersionErr = clierrors.NewValidationError("stop partial chain")
		first, err := Commit(api, CommitOptions{PlanPath: planPath})
		if err == nil || first.Aborted == nil || first.Aborted.Ordinal != 2 {
			t.Fatalf("expected partial export chain: summary=%#v err=%v", first, err)
		}
		api.failVersionErr = nil

		writeWorkspaceExportForTest(t, exportPath, "after", 2)
		plan, _, err := Propose(api, options)
		if err != nil {
			t.Fatal(err)
		}
		if plan.Manifest[0].Action != ActionSkip ||
			!exceptionDetailContains(plan.Exceptions, "different-revision partial chain") {
			t.Fatalf("different-revision partial chain was not skipped: %#v", plan)
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
		_, _, _, err = Review(CommitOptions{PlanPath: planPath})
		if err == nil || !strings.Contains(err.Error(), "different-revision partial chain may only remain skipped") {
			t.Fatalf("review must reject a write action for a different partial revision: %v", err)
		}
	})
}

func proposeLinkedMarkdown(t *testing.T) (*testAPI, string) {
	t.Helper()
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "a.md"), "# A\n\nSee [B](b.md).\n")
	writeTestFile(t, filepath.Join(source, "b.md"), "# B\n\nSee [[A]].\n")
	planPath := filepath.Join(root, "import.plan.yaml")
	api := newTestAPI()
	if _, _, err := Propose(api, ProposeOptions{
		SourcePath: source, PlanPath: planPath, Workspace: 1,
	}); err != nil {
		t.Fatal(err)
	}
	return api, planPath
}

func writeWorkspaceExportForTest(t *testing.T, path, latestBody string, versions int) {
	t.Helper()
	if versions == 1 {
		writeTestFile(t, path, `{
  "format": "recuerd0.workspace_export",
  "format_version": 1,
  "workspace": {"id": 22},
  "memories": [{
    "root_id": 7,
    "versions": [
      {"version": 1, "title": "Chain", "body": "`+latestBody+`", "tags": ["b", "a"], "category": "general"}
    ]
  }]
}`)
		return
	}
	writeTestFile(t, path, `{
  "format": "recuerd0.workspace_export",
  "format_version": 1,
  "workspace": {"id": 22},
  "memories": [{
    "root_id": 7,
    "versions": [
      {"version": 1, "title": "Chain", "body": "one", "tags": ["b", "a"], "category": "general"},
      {"version": 2, "title": "Chain", "body": "`+latestBody+`", "tags": ["b", "a"], "category": "decision"}
    ]
  }]
}`)
}

func exceptionDetailContains(exceptions []Exception, fragment string) bool {
	for _, exception := range exceptions {
		if strings.Contains(exception.Detail, fragment) {
			return true
		}
	}
	return false
}
