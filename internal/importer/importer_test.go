package importer

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/maquina/recuerd0-cli/internal/client"
	clierrors "github.com/maquina/recuerd0-cli/internal/errors"
	"gopkg.in/yaml.v3"
)

type testAPI struct {
	memories       map[int64]memoryRecord
	links          map[[2]int64]bool
	nextID         int64
	getCalls       []string
	postCalls      []testCall
	failVersion    int
	failVersionErr error
}

type testCall struct {
	path string
	body interface{}
}

func newTestAPI() *testAPI {
	return &testAPI{memories: make(map[int64]memoryRecord), links: make(map[[2]int64]bool), nextID: 100}
}

func (api *testAPI) Get(path string) (*client.APIResponse, error) {
	api.getCalls = append(api.getCalls, path)
	if strings.HasSuffix(path, "/links") {
		id := pathID(path)
		var items []interface{}
		for pair := range api.links {
			var other int64
			switch id {
			case pair[0]:
				other = pair[1]
			case pair[1]:
				other = pair[0]
			default:
				continue
			}
			items = append(items, memoryData(api.memories[other]))
		}
		return &client.APIResponse{StatusCode: 200, Data: items}, nil
	}
	id := pathID(path)
	memory, ok := api.memories[id]
	if !ok {
		return nil, clierrors.NewNotFoundError("not found")
	}
	return &client.APIResponse{StatusCode: 200, Data: memoryData(memory)}, nil
}

func (api *testAPI) GetWithPagination(path string) (*client.APIResponse, error) {
	api.getCalls = append(api.getCalls, path)
	items := make([]interface{}, 0, len(api.memories))
	for id := int64(1); id <= api.nextID+100; id++ {
		if memory, ok := api.memories[id]; ok {
			items = append(items, memoryData(memory))
		}
	}
	return &client.APIResponse{StatusCode: 200, Data: items}, nil
}

func (api *testAPI) Post(path string, body interface{}) (*client.APIResponse, error) {
	api.postCalls = append(api.postCalls, testCall{path: path, body: body})
	if strings.HasSuffix(path, "/links") {
		from := pathID(path)
		to := body.(map[string]interface{})["to_memory_id"].(int64)
		pair := [2]int64{from, to}
		if pair[0] > pair[1] {
			pair[0], pair[1] = pair[1], pair[0]
		}
		if api.links[pair] {
			return nil, clierrors.NewValidationError("already linked")
		}
		api.links[pair] = true
		return &client.APIResponse{StatusCode: 201, Data: map[string]interface{}{}}, nil
	}
	if strings.HasSuffix(path, "/memories") {
		api.nextID++
		values := body.(map[string]interface{})["memory"].(map[string]interface{})
		memory := memoryFromPayload(api.nextID, 1, values)
		api.memories[memory.ID] = memory
		return &client.APIResponse{StatusCode: 201, Data: memoryData(memory)}, nil
	}
	id := pathID(strings.TrimSuffix(path, "/versions"))
	current := api.memories[id]
	nextVersion := current.Version + 1
	if api.failVersion == nextVersion && api.failVersionErr != nil {
		return nil, api.failVersionErr
	}
	values := body.(map[string]interface{})["version"].(map[string]interface{})
	memory := memoryFromPayload(id, nextVersion, values)
	memory.Source = current.Source
	api.memories[id] = memory
	// Deliberately return a version-row ID. Import must ignore it.
	return &client.APIResponse{StatusCode: 201, Data: map[string]interface{}{"id": 9000 + nextVersion, "version": nextVersion}}, nil
}

func (api *testAPI) Patch(string, interface{}) (*client.APIResponse, error) {
	panic("unexpected Patch")
}

func (api *testAPI) Delete(string) (*client.APIResponse, error) {
	panic("unexpected Delete")
}

func memoryFromPayload(id int64, version int, values map[string]interface{}) memoryRecord {
	tags, _ := values["tags"].([]string)
	return memoryRecord{
		ID: id, Version: version,
		Title: values["title"].(string), Body: values["content"].(string),
		Tags: append([]string(nil), tags...), Category: values["category"].(string),
		Source: stringValue(values["source"]),
	}
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return text
}

func memoryData(memory memoryRecord) map[string]interface{} {
	return map[string]interface{}{
		"id": memory.ID, "title": memory.Title, "version": memory.Version,
		"source": memory.Source, "tags": append([]string(nil), memory.Tags...),
		"category": memory.Category, "content": map[string]interface{}{"body": memory.Body},
	}
}

func pathID(path string) int64 {
	trimmed := strings.TrimSuffix(strings.SplitN(path, "?", 2)[0], ".json")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		id, err := strconv.ParseInt(parts[i], 10, 64)
		if err == nil {
			return id
		}
	}
	return 0
}

func TestMarkdownScanDuplicateScopeAndMetadata(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "---\ntags: Alpha, two-words\ncategory: decision\n---\n# Shared\n\nSame body\n")
	writeTestFile(t, filepath.Join(root, "nested", "z-copy.md"), "---\ntags:\n  - Beta Tag\n---\n# Shared\n\nSame body\n")
	writeTestFile(t, filepath.Join(root, "third.md"), "No H1\n")

	result, err := scanMarkdown(root, defaultRules())
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(result.Rows))
	}
	var first, copy sourceRow
	for _, row := range result.Rows {
		switch row.Path {
		case "a.md":
			first = row
		case "nested/z-copy.md":
			copy = row
		}
	}
	if first.Category != "decision" || !reflect.DeepEqual(first.Tags, []string{"alpha", "two_words"}) {
		t.Fatalf("unexpected first metadata: %#v", first)
	}
	if !reflect.DeepEqual(copy.Tags, []string{"beta_tag", "nested"}) {
		t.Fatalf("unexpected copy tags: %#v", copy.Tags)
	}
	if hasException(first.Exceptions, "dupe_exact") {
		t.Fatal("earliest duplicate must not receive dupe_exact")
	}
	if !hasException(copy.Exceptions, "dupe_exact") {
		t.Fatal("later duplicate must receive dupe_exact")
	}
}

func TestProposeDeterministicAndStickyEditedRow(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "notes", "one.md"), "# Original\n\nBody\n")
	planPath := filepath.Join(root, "import.plan.yaml")
	api := newTestAPI()
	options := ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 9}
	if _, _, err := Propose(api, options); err != nil {
		t.Fatal(err)
	}
	firstBytes, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := Propose(api, options); err != nil {
		t.Fatal(err)
	}
	secondBytes, _ := os.ReadFile(planPath)
	if string(firstBytes) != string(secondBytes) {
		t.Fatal("untouched propose must be byte deterministic")
	}

	plan, err := LoadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	oldFingerprint := plan.Manifest[0].RowFingerprint
	oldContentHash := plan.Manifest[0].ContentHash
	plan.Manifest[0].Title = "Reviewed title"
	plan.Manifest[0].Tags = []string{"manual"}
	plan.Rules.TagMap = map[string][]string{"notes": {"changed"}}
	if err := SavePlanAtomic(planPath, plan); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Propose(api, options); err != nil {
		t.Fatal(err)
	}
	after, _ := LoadPlan(planPath)
	row := after.Manifest[0]
	if row.Title != "Reviewed title" || !reflect.DeepEqual(row.Tags, []string{"manual"}) {
		t.Fatalf("reviewed scanner fields were not sticky: %#v", row)
	}
	if row.RowFingerprint != oldFingerprint {
		t.Fatal("edited row fingerprint must remain byte-for-byte unchanged")
	}
	wantContentHash := CanonicalTupleHash("Reviewed title", []string{"manual"}, "general", "# Original\n\nBody\n")
	if row.ContentHash != wantContentHash || row.ContentHash == oldContentHash {
		t.Fatalf("re-propose must refresh hashes for preserved scanner edits: got %s want %s", row.ContentHash, wantContentHash)
	}
	if countString(row.Notes, "row edited — rules changes not applied") != 1 {
		t.Fatalf("edited-row note must appear exactly once: %#v", row.Notes)
	}
	after.Rules.TagMap = map[string][]string{"notes": {"changed-again"}}
	if err := SavePlanAtomic(planPath, after); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Propose(api, options); err != nil {
		t.Fatal(err)
	}
	again, _ := LoadPlan(planPath)
	if again.Manifest[0].RowFingerprint != oldFingerprint ||
		countString(again.Manifest[0].Notes, "row edited — rules changes not applied") != 1 {
		t.Fatal("sticky edit provenance changed on a second re-propose")
	}
}

func TestProposeIgnoresPlanAndLedgerInsideMarkdownSource(t *testing.T) {
	source := t.TempDir()
	writeTestFile(t, filepath.Join(source, "one.md"), "# One\n\nBody\n")
	planPath := filepath.Join(source, "import.plan.yaml")
	ledgerPath := filepath.Join(source, "import.ledger.jsonl")
	api := newTestAPI()
	options := ProposeOptions{
		SourcePath: source, PlanPath: planPath, LedgerPath: ledgerPath, Workspace: 2,
	}
	if _, _, err := Propose(api, options); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(planPath)
	if _, _, err := Propose(api, options); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(planPath)
	if string(first) != string(second) {
		t.Fatal("plan/ledger artifacts inside the source must not perturb re-propose")
	}
	plan, _ := LoadPlan(planPath)
	if plan.Scan.Excluded != 0 || len(plan.Scan.Warnings) != 0 {
		t.Fatalf("import artifacts should not count as exclusions: %#v", plan.Scan)
	}
}

func TestLoadPlanReportsLineAndAgreementLocations(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "one.md"), "# One\n\nBody\n")
	planPath := filepath.Join(root, "import.plan.yaml")
	api := newTestAPI()
	if _, _, err := Propose(api, ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 1}); err != nil {
		t.Fatal(err)
	}
	plan, _ := LoadPlan(planPath)
	plan.Exceptions = append(plan.Exceptions, Exception{Path: "one.md", Kind: "conflict", Resolution: ActionSkip})
	// Marshal directly because SavePlanAtomic correctly refuses disagreement.
	yamlBytes, err := yaml.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, planPath, string(yamlBytes))
	_, err = LoadPlan(planPath)
	if err == nil || !strings.Contains(err.Error(), "action/resolution disagreement") || !strings.Contains(err.Error(), "line ") {
		t.Fatalf("expected line-numbered agreement error, got %v", err)
	}
}

func TestCommitCreateVerifiesAndPersistsFixedIdentity(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "one.md"), "---\ntags: z,a\ncategory: discovery\n---\n# One\n\nBody\n")
	planPath := filepath.Join(root, "import.plan.yaml")
	api := newTestAPI()
	if _, _, err := Propose(api, ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 4}); err != nil {
		t.Fatal(err)
	}
	summary, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Ops.Created != 1 || !summary.Plan.Complete {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	ledger, err := LoadLedger(filepath.Join(root, "import.ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Records) != 2 || ledger.Records[1].MemoryID != 101 ||
		ledger.Records[1].Version != 1 || ledger.Records[1].ChainBase != 0 {
		t.Fatalf("unexpected ledger: %#v", ledger.Records)
	}
	rawLedger, _ := os.ReadFile(filepath.Join(root, "import.ledger.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(rawLedger)), "\n")
	if strings.Contains(lines[0], "committed_at") || strings.Contains(lines[1], "attempted_at") {
		t.Fatalf("ledger records leaked fields from the other kind:\n%s", rawLedger)
	}
	payload := api.postCalls[0].body.(map[string]interface{})["memory"].(map[string]interface{})
	for _, key := range []string{"title", "content", "tags", "category", "source"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("create payload omitted %s: %#v", key, payload)
		}
	}
}

func TestExportAppendResumeKeepsBaseAndIgnoresVersionIDs(t *testing.T) {
	root := t.TempDir()
	exportPath := filepath.Join(root, "export.json")
	writeTestFile(t, exportPath, `{
  "format": "recuerd0.workspace_export",
  "format_version": 1,
  "workspace": {"id": 22},
  "memories": [{
    "root_id": 7,
    "versions": [
      {"version": 1, "title": "Chain", "body": "one", "tags": ["b", "a"], "category": "general"},
      {"version": 2, "title": "Chain", "body": "two", "tags": ["b", "a"], "category": "decision"},
      {"version": 3, "title": "Chain", "body": "three", "tags": ["b", "a"], "category": "discovery"}
    ]
  }]
}`)
	api := newTestAPI()
	api.memories[50] = memoryRecord{
		ID: 50, Title: "Chain", Body: "existing", Tags: []string{"old"},
		Category: "general", Source: "manual", Version: 7,
	}
	planPath := filepath.Join(root, "import.plan.yaml")
	if _, _, err := Propose(api, ProposeOptions{
		SourcePath: exportPath, PlanPath: planPath, Workspace: 3,
		Adapter: AdapterWorkspaceExport,
	}); err != nil {
		t.Fatal(err)
	}
	plan, _ := LoadPlan(planPath)
	plan.Manifest[0].Action = ActionVersion
	for i := range plan.Exceptions {
		if plan.Exceptions[i].Path == plan.Manifest[0].Path {
			plan.Exceptions[i].Resolution = ActionVersion
		}
	}
	if err := SavePlanAtomic(planPath, plan); err != nil {
		t.Fatal(err)
	}

	api.failVersion = 10
	api.failVersionErr = clierrors.NewValidationError("injected stop")
	first, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err == nil || first.Aborted == nil || first.Aborted.Ordinal != 3 {
		t.Fatalf("expected ordinal-3 abort, got summary=%#v err=%v", first, err)
	}
	api.failVersionErr = nil
	prior := api.memories[50]
	advanced := prior
	advanced.Version = 11
	api.memories[50] = advanced
	concurrent, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err == nil || concurrent.Aborted == nil ||
		concurrent.Aborted.Path != plan.Manifest[0].Path || concurrent.Aborted.Ordinal != 3 {
		t.Fatalf("concurrent resume must name row and ordinal: summary=%#v err=%v", concurrent, err)
	}
	api.memories[50] = prior
	second, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if !second.Plan.Complete || api.memories[50].Version != 10 {
		t.Fatalf("resume did not complete at version 10: %#v memory=%#v", second, api.memories[50])
	}
	ledger, err := LoadLedger(filepath.Join(root, "import.ledger.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	intentCount := 0
	for _, record := range ledger.Records {
		if record.Kind == "intent" {
			intentCount++
		}
		if record.ChainBase != 7 {
			t.Fatalf("chain base changed on resume: %#v", record)
		}
		if record.MemoryID != 0 && record.MemoryID != 50 {
			t.Fatalf("version response ID entered ledger identity: %#v", record)
		}
	}
	versionPosts := 0
	for _, call := range api.postCalls {
		if strings.HasSuffix(call.path, "/versions") {
			versionPosts++
		}
	}
	if intentCount != versionPosts {
		t.Fatalf("every pass-1 POST needs a preceding intent: intents=%d posts=%d", intentCount, versionPosts)
	}
}

func TestReviewDetectsSourceDrift(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	file := filepath.Join(source, "one.md")
	writeTestFile(t, file, "# One\n\nBefore\n")
	planPath := filepath.Join(root, "import.plan.yaml")
	api := newTestAPI()
	if _, _, err := Propose(api, ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 1}); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, file, "# One\n\nAfter\n")
	_, _, _, err := Review(CommitOptions{PlanPath: planPath})
	if err == nil || !strings.Contains(err.Error(), "source changed since propose — re-run propose") {
		t.Fatalf("expected canonical drift error, got %v", err)
	}
}

func TestCommitLinksAreCanonicalAndRerunReconcilesExisting(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "a.md"), "# A\n\nSee [B](b.md).\n")
	writeTestFile(t, filepath.Join(source, "b.md"), "# B\n\nSee [[A]].\n")
	planPath := filepath.Join(root, "import.plan.yaml")
	api := newTestAPI()
	if _, _, err := Propose(api, ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 1}); err != nil {
		t.Fatal(err)
	}
	first, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if first.LinksEnsured.Created != 1 || !first.Plan.Complete {
		t.Fatalf("expected one canonical link: %#v", first)
	}
	second, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if second.LinksEnsured.Existing != 1 || !second.Plan.Complete {
		t.Fatalf("rerun should reconcile existing link: %#v", second)
	}
}

func TestLedgerSupportsCategoryOnlyMarkdownRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.jsonl")
	sourceHash := sourceBodyHash("same body")
	firstHash := CanonicalTupleHash("T", nil, "general", "same body")
	secondHash := CanonicalTupleHash("T", nil, "decision", "same body")
	firstIntent := newIntent("a.md", 1, ActionCreate, 1, 1, sourceHash, firstHash, 1, 0, 0)
	appendRecordForTest(t, path, firstIntent)
	appendRecordForTest(t, path, newCommitted(firstIntent, 9, false))
	secondIntent := newIntent("a.md", 1, ActionVersion, 1, 1, sourceHash, secondHash, 2, 1, 9)
	appendRecordForTest(t, path, secondIntent)
	appendRecordForTest(t, path, newCommitted(secondIntent, 9, false))
	ledger, err := LoadLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.Paths["a.md"].Revisions) != 2 {
		t.Fatalf("expected two revisions sharing a source hash, got %#v", ledger.Paths["a.md"].Revisions)
	}
}

func TestCanonicalImportWording(t *testing.T) {
	const agentBoundary = "The agent's job ends at the plan. Never import by writing memories one-by-one through MCP; always execute through `recuerdo import commit`, and pass `--yes` only after the human has seen the digest and said go."
	const scannerReviewProtocol = "After editing any scanner-owned field (`title`, `category`, `tags`, or `links`), re-run"
	if ThinHint != "This plan looks thin — refine it by hand or hand it to your agent (see the recuerd0 skill's import protocol)." {
		t.Fatalf("thin hint changed: %q", ThinHint)
	}
	for _, relative := range []string{
		"../../docs/IMPORT.md",
		"../../docs/claude-code-guide.md",
		"../../skills/recuerd0/SKILL.md",
	} {
		data, err := os.ReadFile(relative)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), agentBoundary) {
			t.Fatalf("%s does not contain the canonical agent boundary", relative)
		}
		if strings.HasSuffix(relative, "IMPORT.md") || strings.HasSuffix(relative, "SKILL.md") {
			if !strings.Contains(string(data), scannerReviewProtocol) {
				t.Fatalf("%s does not contain the scanner-owned re-propose review protocol", relative)
			}
		}
	}
}

func appendRecordForTest(t *testing.T, path string, record LedgerRecord) {
	t.Helper()
	record.AttemptedAt = truncateTime(record.AttemptedAt)
	record.CommittedAt = truncateTime(record.CommittedAt)
	if err := AppendLedgerRecord(path, record); err != nil {
		t.Fatal(err)
	}
}

func truncateTime(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return value.Truncate(time.Microsecond)
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasException(exceptions []Exception, kind string) bool {
	for _, exception := range exceptions {
		if exception.Kind == kind {
			return true
		}
	}
	return false
}

func countString(values []string, wanted string) int {
	count := 0
	for _, value := range values {
		if value == wanted {
			count++
		}
	}
	return count
}
