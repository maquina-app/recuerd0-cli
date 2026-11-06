package importer

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
)

func exceptionsFor(plan *Plan, path string) map[string]Exception {
	found := make(map[string]Exception)
	for _, exception := range plan.Exceptions {
		if exception.Path == path {
			found[exception.Kind] = exception
		}
	}
	return found
}

func rowFor(t *testing.T, plan *Plan, path string) PlanRow {
	t.Helper()
	for _, row := range plan.Manifest {
		if row.Path == path {
			return row
		}
	}
	t.Fatalf("no row for %s", path)
	return PlanRow{}
}

func manyTags(count int) string {
	var builder strings.Builder
	builder.WriteString("---\ntags:\n")
	for i := 0; i < count; i++ {
		fmt.Fprintf(&builder, "  - tag%d\n", i)
	}
	builder.WriteString("---\n")
	return builder.String()
}

func TestProposeSkipsMarkdownTheServerRefusesAndCommitImportsTheRest(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "fifty-tags.md"), manyTags(50)+"# Fifty\n\nAllowed.\n")
	writeTestFile(t, filepath.Join(source, "fifty-one-tags.md"), manyTags(51)+"# Fifty one\n\nToo many.\n")
	writeTestFile(t, filepath.Join(source, "long-tag.md"), "---\ntags:\n  - "+strings.Repeat("y", 101)+"\n---\n# Long tag\n\nBody.\n")
	writeTestFile(t, filepath.Join(source, "max-tag.md"), "---\ntags:\n  - "+strings.Repeat("é", 100)+"\n---\n# Max tag\n\nCounted in characters.\n")
	writeTestFile(t, filepath.Join(source, "big.md"), "# Big\n\n"+strings.Repeat("x", maxContentBytes))
	writeTestFile(t, filepath.Join(source, "null.md"), "# Null\n\nA \x00 byte.\n")
	writeTestFile(t, filepath.Join(source, "zz-normal.md"), "# Normal\n\nBody.\n")

	api := newTestAPI()
	planPath := filepath.Join(root, "import.plan.yaml")
	plan, digest, err := Propose(api, ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 3})
	if err != nil {
		t.Fatal(err)
	}
	for path, want := range map[string]string{
		"fifty-one-tags.md": "51 tags (maximum 50)",
		"long-tag.md":       "a tag of 101 characters (maximum 100)",
		"big.md":            "bytes of content (maximum 1048576)",
		"null.md":           "a null character",
	} {
		exception, ok := exceptionsFor(plan, path)["unimportable"]
		if !ok || exception.Resolution != ActionSkip || !strings.Contains(exception.Detail, want) {
			t.Errorf("%s: expected a skip naming %q, got %#v", path, want, exception)
		}
		if action := rowFor(t, plan, path).Action; action != ActionSkip {
			t.Errorf("%s: expected skip, got %s", path, action)
		}
	}
	for _, path := range []string{"fifty-tags.md", "max-tag.md", "zz-normal.md"} {
		if action := rowFor(t, plan, path).Action; action != ActionCreate {
			t.Errorf("%s: at the limit is allowed, got %s", path, action)
		}
	}
	if digest.Counts.Create != 3 || digest.Counts.Skip != 4 {
		t.Fatalf("digest doesn't show the skips: %#v", digest.Counts)
	}

	summary, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Ops.Created != 3 || summary.Plan.RowsReviewSkip != 4 || !summary.Plan.Complete {
		t.Fatalf("the rest of the vault should land: %#v", summary)
	}
}

func TestProposeCountsTagsTheWayTheServerDoes(t *testing.T) {
	tags := make([]string, 0, 60)
	for i := 0; i < 30; i++ {
		tags = append(tags, fmt.Sprintf("Tag%d", i), fmt.Sprintf(" tag%d ", i))
	}
	if problems := contentProblems(tags, "body"); len(problems) != 0 {
		t.Fatalf("30 tags repeated in another case are 30 tags, got %v", problems)
	}
}

func TestTrimmingTagsInThePlanClearsTheLimitException(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "many.md"), manyTags(51)+"# Many\n\nBody.\n")
	api := newTestAPI()
	planPath := filepath.Join(root, "import.plan.yaml")
	options := ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 3}
	plan, _, err := Propose(api, options)
	if err != nil {
		t.Fatal(err)
	}
	plan.Manifest[0].Tags = plan.Manifest[0].Tags[:50]
	if err := SavePlanAtomic(planPath, plan); err != nil {
		t.Fatal(err)
	}
	plan, _, err = Propose(api, options)
	if err != nil {
		t.Fatal(err)
	}
	if _, flagged := exceptionsFor(plan, "many.md")["unimportable"]; flagged {
		t.Fatalf("reviewed tags are within the limit: %#v", plan.Exceptions)
	}
}

type exportVersion struct {
	Version  int      `json:"version"`
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Tags     []string `json:"tags"`
	Category string   `json:"category"`
}

func proposeExportChain(t *testing.T, api client.API, versions ...exportVersion) (*Plan, string, string) {
	t.Helper()
	root := t.TempDir()
	for i := range versions {
		versions[i].Version = i + 1
		versions[i].Category = "general"
	}
	data, err := json.Marshal(map[string]interface{}{
		"format": "recuerd0.workspace_export", "format_version": 1,
		"workspace": map[string]interface{}{"id": 22},
		"memories":  []interface{}{map[string]interface{}{"root_id": 7, "versions": versions}},
	})
	if err != nil {
		t.Fatal(err)
	}
	exportPath := filepath.Join(root, "export.json")
	writeTestFile(t, exportPath, string(data))
	planPath := filepath.Join(root, "import.plan.yaml")
	plan, _, err := Propose(api, ProposeOptions{
		SourcePath: exportPath, PlanPath: planPath, Workspace: 3, Adapter: AdapterWorkspaceExport,
	})
	if err != nil {
		t.Fatal(err)
	}
	return plan, planPath, exportPath
}

const exportRowPath = "workspace/22/memories/7"

func TestExportChainDropsWhatTheServerRefuses(t *testing.T) {
	v := func(title, body string, tags ...string) exportVersion {
		return exportVersion{Title: title, Body: body, Tags: tags}
	}
	tests := []struct {
		name     string
		versions []exportVersion
		landed   int
		lastBody string
		kinds    []string
		detail   string
	}{
		{"nothing to drop", []exportVersion{v("T", "one"), v("T", "two")}, 2, "two", nil, ""},
		{"repeat", []exportVersion{v("T", "same"), v("T", "same"), v("T", "next")}, 2, "next", []string{"repeated_versions"}, "version(s) 2 repeat"},
		{"repeat once repeated tags are dropped", []exportVersion{v("T", "same", "Ops"), v("T", "same", "Ops", "ops")}, 1, "same", []string{"repeated_versions"}, ""},
		{"a tag's spelling changing is not a repeat", []exportVersion{v("T", "same", "Ops"), v("T", "same", "ops")}, 2, "same", nil, ""},
		{"repeat once fences are separated", []exportVersion{v("T", "- a\n```\nx\n```\n"), v("T", "- a\n\n```\nx\n```\n")}, 1, "- a\n```\nx\n```\n", []string{"repeated_versions"}, ""},
		{"a title change is not a repeat", []exportVersion{v("T", "same"), v("U", "same")}, 2, "same", nil, ""},
		{"blank in the middle", []exportVersion{v("T", "one"), v("T", ""), v("T", "two")}, 2, "two", []string{"blank_versions"}, "version(s) 2 are blank"},
		{"blank at the end", []exportVersion{v("T", "one"), v("T", " \n")}, 1, "one", []string{"blank_versions"}, "ends on the last one with content"},
		{"blank between identical versions", []exportVersion{v("T", "one"), v("T", ""), v("T", "one")}, 1, "one", []string{"blank_versions", "repeated_versions"}, ""},
		{"empty first version", []exportVersion{v("", ""), v("T", "one")}, 1, "one", []string{"blank_versions"}, ""},
		{"title-only first version stays", []exportVersion{v("T", ""), v("T", "one")}, 2, "one", nil, ""},
		{"blank after blank isn't an overwrite", []exportVersion{v("T", ""), v("U", "")}, 2, "", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newTestAPI()
			plan, planPath, _ := proposeExportChain(t, api, tt.versions...)
			exceptions := exceptionsFor(plan, exportRowPath)
			if len(exceptions) != len(tt.kinds) {
				t.Fatalf("expected %v, got %#v", tt.kinds, exceptions)
			}
			for _, kind := range tt.kinds {
				exception, ok := exceptions[kind]
				if !ok || exception.Resolution != ActionCreate || !strings.Contains(exception.Detail, tt.detail) {
					t.Fatalf("expected %s naming %q, got %#v", kind, tt.detail, exceptions)
				}
			}
			summary, err := Commit(api, CommitOptions{PlanPath: planPath})
			if err != nil {
				t.Fatal(err)
			}
			got := api.memories[101]
			if summary.Ops.Created+summary.Ops.Versioned != tt.landed || got.Version != tt.landed ||
				got.Body != tt.lastBody || !summary.Plan.Complete {
				t.Fatalf("expected %d versions ending %q, got %#v and %#v", tt.landed, tt.lastBody, got, summary)
			}
		})
	}
}

func TestExportWithNothingToImportIsSkipped(t *testing.T) {
	api := newTestAPI()
	plan, planPath, _ := proposeExportChain(t, api, exportVersion{}, exportVersion{})
	if _, ok := exceptionsFor(plan, exportRowPath)["unimportable"]; !ok {
		t.Fatalf("expected a skip: %#v", plan.Exceptions)
	}
	summary, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if len(api.postCalls) != 0 || !summary.Plan.Complete {
		t.Fatalf("nothing should be written: %#v", summary)
	}
}

func TestEmptyExportOverALimitIsFlaggedOnce(t *testing.T) {
	tooMany := make([]string, 51)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("tag%d", i)
	}
	plan, _, _ := proposeExportChain(t, newTestAPI(), exportVersion{Tags: tooMany})
	if len(plan.Exceptions) != 1 || plan.Exceptions[0].Kind != "unimportable" {
		t.Fatalf("expected one exception, got %#v", plan.Exceptions)
	}
}

func TestExportOverALimitSkipsTheWholeMemory(t *testing.T) {
	tooMany := make([]string, 51)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("tag%d", i)
	}
	tests := []struct {
		name     string
		versions []exportVersion
		kind     string
		detail   string
	}{
		{"tags in a later version", []exportVersion{{Title: "T", Body: "one"}, {Title: "T", Body: "two", Tags: tooMany}}, "unimportable", "version 2 has 51 tags"},
		{"long tag", []exportVersion{{Title: "T", Body: "one", Tags: []string{strings.Repeat("y", 101)}}}, "unimportable", "version 1 has a tag of 101"},
		{"content", []exportVersion{{Title: "T", Body: strings.Repeat("x", maxContentBytes+1)}}, "unimportable", "bytes of content"},
		{"title", []exportVersion{{Title: strings.Repeat("t", 256), Body: "one"}}, "title_too_long", "title is 256 characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newTestAPI()
			plan, planPath, _ := proposeExportChain(t, api, tt.versions...)
			exception, ok := exceptionsFor(plan, exportRowPath)[tt.kind]
			if !ok || exception.Resolution != ActionSkip || !strings.Contains(exception.Detail, tt.detail) {
				t.Fatalf("expected %s naming %q, got %#v", tt.kind, tt.detail, plan.Exceptions)
			}
			if _, err := Commit(api, CommitOptions{PlanPath: planPath}); err != nil {
				t.Fatal(err)
			}
			if len(api.postCalls) != 0 {
				t.Fatalf("nothing should be written, got %#v", api.postCalls)
			}
		})
	}
}

func TestStoredBodyMatchesTheServersNormalizer(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"fence after a list", "- a\n```\nx\n```\n", "- a\n\n```\nx\n```\n"},
		{"tilde fence after an ordered list", "1) a\n~~~\nx\n~~~\n", "1) a\n\n~~~\nx\n~~~\n"},
		{"already separated", "- a\n\n```\nx\n```\n", "- a\n\n```\nx\n```\n"},
		{"list continuation still counts", "- a\n  more\n```\nx\n```\n", "- a\n  more\n\n```\nx\n```\n"},
		{"indented fence belongs to its item", "- a\n  ```\n  x\n  ```\n", "- a\n  ```\n  x\n  ```\n"},
		{"list inside a fence", "```\n- a\n```\n", "```\n- a\n```\n"},
		{"a fence ends the list", "- a\n```\nx\n```\n```\ny\n```\n", "- a\n\n```\nx\n```\n```\ny\n```\n"},
		{"no fence", "- a\n- b", "- a\n- b"},
		{"crlf", "- a\r\n```\r\nx\r\n```\r\n", "- a\r\n\n```\r\nx\r\n```\r\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storedBody(tt.in)
			if got != tt.want {
				t.Fatalf("storedBody(%q) = %q, want %q", tt.in, got, tt.want)
			}
			if again := storedBody(got); again != got {
				t.Fatalf("not idempotent: %q then %q", got, again)
			}
		})
	}
}

// normalizingAPI stores what the real server stores, spelled out by hand so the
// test doesn't lean on the code it checks.
type normalizingAPI struct {
	*testAPI
}

func (api *normalizingAPI) Post(path string, body interface{}) (*client.APIResponse, error) {
	response, err := api.testAPI.Post(path, body)
	for id, memory := range api.memories {
		memory.Body = strings.ReplaceAll(memory.Body, "- item\n```", "- item\n\n```")
		seen := map[string]bool{}
		kept := []string{}
		for _, tag := range memory.Tags {
			if !seen[strings.ToLower(tag)] {
				seen[strings.ToLower(tag)] = true
				kept = append(kept, tag)
			}
		}
		memory.Tags = kept
		api.memories[id] = memory
	}
	return response, err
}

func TestCommitAcceptsWhatTheServerNormalizes(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "fence.md"), "# Fence\n\n- item\n```\ncode\n```\n")
	writeTestFile(t, filepath.Join(source, "other.md"), "# Other\n\nBody.\n")
	api := &normalizingAPI{newTestAPI()}
	planPath := filepath.Join(root, "import.plan.yaml")
	options := ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 3}
	plan, _, err := Propose(api, options)
	if err != nil {
		t.Fatal(err)
	}
	for i := range plan.Manifest {
		plan.Manifest[i].Tags = []string{"Ops", "ops"}
	}
	if err := SavePlanAtomic(planPath, plan); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Propose(api, options); err != nil {
		t.Fatal(err)
	}
	summary, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatalf("a normalized read-back must not stop the import: %v (%#v)", err, summary.Aborted)
	}
	if summary.Ops.Created != 2 || !summary.Plan.Complete {
		t.Fatalf("both rows should land: %#v", summary)
	}
}

func TestVersionRowTheServerWouldRefuseFailsBeforeAnyWrite(t *testing.T) {
	tests := []struct {
		name, body, want string
	}{
		{"same as the target", "# Note\n\nSame.\n", "would change nothing"},
		{"blank over content", "", "has no content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "vault")
			writeTestFile(t, filepath.Join(source, "first.md"), "# First\n\nBody.\n")
			writeTestFile(t, filepath.Join(source, "note.md"), tt.body)
			api := newTestAPI()
			api.memories[5] = memoryRecord{ID: 5, Title: "Note", Body: "# Note\n\nSame.\n", Category: "general", Version: 1, Tags: []string{}}
			if tt.body == "" {
				api.memories[5] = memoryRecord{ID: 5, Title: "note", Body: "Kept.", Category: "general", Version: 1, Tags: []string{}}
			}
			planPath := filepath.Join(root, "import.plan.yaml")
			plan, _, err := Propose(api, ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 3})
			if err != nil {
				t.Fatal(err)
			}
			for i := range plan.Manifest {
				if plan.Manifest[i].Path == "note.md" {
					plan.Manifest[i].Action = ActionVersion
				}
			}
			for i := range plan.Exceptions {
				if plan.Exceptions[i].Path == "note.md" {
					plan.Exceptions[i].Resolution = ActionVersion
				}
			}
			if err := SavePlanAtomic(planPath, plan); err != nil {
				t.Fatal(err)
			}
			_, err = Commit(api, CommitOptions{PlanPath: planPath})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q, got %v", tt.want, err)
			}
			if len(api.postCalls) != 0 {
				t.Fatalf("preflight must stop before writing: %#v", api.postCalls)
			}
		})
	}
}

func TestFileEmptiedSinceItsImportIsSkippedAndTheRestGoesAhead(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "note.md"), "# Note\n\nBody.\n")
	api := newTestAPI()
	planPath := filepath.Join(root, "import.plan.yaml")
	options := ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 3}
	if _, _, err := Propose(api, options); err != nil {
		t.Fatal(err)
	}
	if _, err := Commit(api, CommitOptions{PlanPath: planPath}); err != nil {
		t.Fatal(err)
	}

	writeTestFile(t, filepath.Join(source, "note.md"), "")
	writeTestFile(t, filepath.Join(source, "new.md"), "# New\n\nBody.\n")
	plan, _, err := Propose(api, options)
	if err != nil {
		t.Fatal(err)
	}
	exception, ok := exceptionsFor(plan, "note.md")["unimportable"]
	if !ok || !strings.Contains(exception.Detail, "blank version") {
		t.Fatalf("expected a skip, got %#v", plan.Exceptions)
	}
	summary, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if summary.Ops.Created != 1 || api.memories[101].Body != "# Note\n\nBody.\n" {
		t.Fatalf("the new file lands and the old text stays: %#v %#v", summary, api.memories[101])
	}
}

func TestPartialChainFromBeforeDroppingStaysSkipped(t *testing.T) {
	api := newTestAPI()
	plan, planPath, exportPath := proposeExportChain(t, api,
		exportVersion{Title: "T", Body: "same"}, exportVersion{Title: "T", Body: "same"}, exportVersion{Title: "T", Body: "next"})
	row := plan.Manifest[0]

	// What an older CLI left behind: the first of three versions, then a refusal.
	ledgerPath := filepath.Join(filepath.Dir(planPath), "import.ledger.jsonl")
	intent := newIntent(row.Path, 3, ActionCreate, 1, 3, row.SourceHash, CanonicalTupleHash("T", nil, "general", "same"), 1, 0, 0)
	if err := AppendLedgerRecord(ledgerPath, intent); err != nil {
		t.Fatal(err)
	}
	if err := AppendLedgerRecord(ledgerPath, newCommitted(intent, 200, false)); err != nil {
		t.Fatal(err)
	}
	api.memories[200] = memoryRecord{ID: 200, Title: "T", Body: "same", Category: "general", Version: 1}

	plan, _, err := Propose(api, ProposeOptions{
		SourcePath: exportPath, PlanPath: planPath, Workspace: 3, Adapter: AdapterWorkspaceExport,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Manifest[0].Action != ActionSkip {
		t.Fatalf("a partial chain of another length has to stay skipped: %#v", plan.Manifest[0])
	}
	posts := len(api.postCalls)
	if _, err := Commit(api, CommitOptions{PlanPath: planPath}); err != nil {
		t.Fatal(err)
	}
	if len(api.postCalls) != posts {
		t.Fatalf("nothing should be written: %#v", api.postCalls)
	}
}
