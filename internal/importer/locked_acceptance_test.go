package importer

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDigestLockedThinFixtures(t *testing.T) {
	t.Run("dump fixture is thin even with folder tags", func(t *testing.T) {
		digest := proposeFixtureDigest(t, filepath.Join("testdata", "digest", "dump"), "")
		if digest.TitlesFromH1Pct != 0 || digest.TagsProposed == 0 {
			t.Fatalf("fixture precondition changed: %#v", digest)
		}
		if !digest.Thin || digest.Hint != ThinHint {
			t.Fatalf("dump fixture must be thin with the locked hint: %#v", digest)
		}
	})

	t.Run("vault fixture is not thin", func(t *testing.T) {
		digest := proposeFixtureDigest(t, filepath.Join("testdata", "digest", "vault"), "")
		if digest.TitlesFromH1Pct != 100 || digest.TagsProposed == 0 {
			t.Fatalf("fixture precondition changed: %#v", digest)
		}
		if digest.Thin || digest.Hint != "" {
			t.Fatalf("vault fixture must not be thin: %#v", digest)
		}
	})

	t.Run("workspace export is never thin", func(t *testing.T) {
		digest := proposeFixtureDigest(t, filepath.Join("testdata", "digest", "workspace-export.json"), AdapterWorkspaceExport)
		if digest.TitlesFromH1Pct != 100 || digest.Thin || digest.Hint != "" {
			t.Fatalf("workspace exports are never thin: %#v", digest)
		}
	})
}

func TestDigestLockedThinFormulaBoundaries(t *testing.T) {
	plan := &Plan{
		Adapter: AdapterObsidianMarkdown,
		Scan:    ScanStats{TitlesTotal: 2, TitlesFromH1: 1},
		Manifest: []PlanRow{{
			Path: "one.md", Action: ActionCreate,
		}},
	}
	if digest := PlanDigest(plan); !digest.Thin || digest.TitlesFromH1Pct != 50 {
		t.Fatalf("50%% H1 with no tags or links must be thin: %#v", digest)
	}
	plan.Manifest[0].Tags = []string{"present"}
	if digest := PlanDigest(plan); digest.Thin {
		t.Fatalf("50%% H1 with a proposed tag must not be thin: %#v", digest)
	}
	plan.Manifest = nil
	if digest := PlanDigest(plan); digest.Thin {
		t.Fatalf("an empty manifest must never be thin: %#v", digest)
	}
}

func TestTagMapFansOutRemovesAndFallsBackToIdentity(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "Team Notes", "one.md"), "---\ntags: [legacy, keep, Unmapped Tag]\n---\n# One\n")
	rules := defaultRules()
	rules.TagMap = map[string][]string{
		"Team Notes": {"area", "Knowledge Base"},
		"legacy":     {},
		"keep":       {"Mapped Tag", "mapped-tag"},
	}
	result, err := scanMarkdown(root, rules)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"area", "knowledge_base", "mapped_tag", "unmapped_tag"}
	if len(result.Rows) != 1 || !reflect.DeepEqual(result.Rows[0].Tags, want) {
		t.Fatalf("tag_map fan-out/removal/identity mismatch: got %#v want %#v", result.Rows, want)
	}
}

func TestMarkdownLockedScannerBehavior(t *testing.T) {
	root := t.TempDir()
	sourceBody := "```md\n# Fenced title\n```\n# Real Title\n\nSee [Target](../TARGET.md), [[Missing]], ![[Diagram]], and ![Pic](image.png).\n"
	writeTestFile(t, filepath.Join(root, "Projects", "Deep", "source.md"),
		"---\ntags: Alpha, Keep-Me\ncategory: invalid\n---\n"+sourceBody)
	writeTestFile(t, filepath.Join(root, "Projects", "target.md"),
		"---\ntags:\n  - Beta Tag\ncategory: decision\n---\n# Target\n")
	writeTestFile(t, filepath.Join(root, "archive", "guess.md"), "No heading here.\n")
	writeTestFile(t, filepath.Join(root, "broken.md"), "---\ntags: [\n# Never parsed\n")
	writeTestFile(t, filepath.Join(root, "long.md"), "# "+strings.Repeat("x", 256)+"\n")
	writeTestFile(t, filepath.Join(root, "attachment.bin"), "binary")
	writeTestFile(t, filepath.Join(root, ".hidden", "ignored.md"), "# Hidden\n")
	writeTestFile(t, filepath.Join(root, "node_modules", "ignored.md"), "# Dependency\n")

	rules := defaultRules()
	rules.CategoryMap = map[string]string{
		"Projects":      "discovery",
		"Projects/Deep": "preference",
	}
	rules.TagMap = map[string][]string{
		"Keep-Me": {"fan out", "Extra"},
	}
	result, err := scanMarkdown(root, rules)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.TitlesTotal != 5 || result.Stats.TitlesFromH1 != 4 ||
		result.Stats.Excluded != 1 ||
		!reflect.DeepEqual(result.Stats.Warnings, []string{"excluded file: attachment.bin"}) {
		t.Fatalf("scanner stats/exclusions mismatch: %#v", result.Stats)
	}

	rows := make(map[string]sourceRow, len(result.Rows))
	orderedPaths := make([]string, 0, len(result.Rows))
	for _, row := range result.Rows {
		rows[row.Path] = row
		orderedPaths = append(orderedPaths, row.Path)
		if strings.HasPrefix(row.Path, "./") || strings.Contains(row.Path, `\`) {
			t.Fatalf("manifest path was not normalized: %q", row.Path)
		}
	}
	if !reflect.DeepEqual(orderedPaths, []string{
		"Projects/Deep/source.md", "Projects/target.md", "archive/guess.md", "broken.md", "long.md",
	}) {
		t.Fatalf("markdown rows were not lexicographically ordered: %#v", orderedPaths)
	}

	source := rows["Projects/Deep/source.md"]
	if source.Title != "Real Title" || !source.FromH1 ||
		source.Category != "preference" ||
		!reflect.DeepEqual(source.Tags, []string{"alpha", "deep", "extra", "fan_out", "projects"}) ||
		!reflect.DeepEqual(source.Links, []string{"Projects/target.md"}) ||
		source.SourceHash != sourceBodyHash(sourceBody) {
		t.Fatalf("nested scanner metadata/link/hash mismatch: %#v", source)
	}
	for _, note := range []string{
		invalidCategoryNote,
		"embed excluded: Diagram",
		"image reference excluded: image.png",
		"unresolved link: Missing",
	} {
		if !containsString(source.Notes, note) {
			t.Fatalf("scanner note %q missing from %#v", note, source.Notes)
		}
	}
	target := rows["Projects/target.md"]
	if target.Category != "decision" || !reflect.DeepEqual(target.Tags, []string{"beta_tag", "projects"}) {
		t.Fatalf("valid frontmatter did not override category map: %#v", target)
	}
	if rows["archive/guess.md"].Title != "guess" ||
		!hasException(rows["archive/guess.md"].Exceptions, "guessed_title") {
		t.Fatalf("filename title fallback mismatch: %#v", rows["archive/guess.md"])
	}
	if !hasException(rows["broken.md"].Exceptions, "unparseable") ||
		!hasException(rows["long.md"].Exceptions, "title_too_long") {
		t.Fatalf("locked scanner exceptions missing: broken=%#v long=%#v",
			rows["broken.md"].Exceptions, rows["long.md"].Exceptions)
	}
}

func TestUntouchedRowRegeneratesUnderTagRuleChanges(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "notes", "one.md"), "# One\n\nBody\n")
	planPath := filepath.Join(root, "import.plan.yaml")
	api := newTestAPI()
	options := ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 1}
	if _, _, err := Propose(api, options); err != nil {
		t.Fatal(err)
	}
	plan, err := LoadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	oldFingerprint := plan.Manifest[0].RowFingerprint
	oldContentHash := plan.Manifest[0].ContentHash
	plan.Rules.TagMap = map[string][]string{"notes": {"changed", "fan out"}}
	if err := SavePlanAtomic(planPath, plan); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Propose(api, options); err != nil {
		t.Fatal(err)
	}
	updated, err := LoadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	row := updated.Manifest[0]
	if !reflect.DeepEqual(row.Tags, []string{"changed", "fan_out"}) ||
		row.RowFingerprint == oldFingerprint || row.ContentHash == oldContentHash {
		t.Fatalf("untouched scanner fields/hashes did not regenerate: %#v", row)
	}
}

func TestSummaryUnresolvableLinkDoesNotPreventCompletion(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "a.md"), "# A\n\nSee [B](b.md).\n")
	writeTestFile(t, filepath.Join(source, "b.md"), "# B\n\nReview-skipped endpoint.\n")
	planPath := filepath.Join(root, "import.plan.yaml")
	api := newTestAPI()
	if _, _, err := Propose(api, ProposeOptions{SourcePath: source, PlanPath: planPath, Workspace: 1}); err != nil {
		t.Fatal(err)
	}
	plan, err := LoadPlan(planPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := range plan.Manifest {
		if plan.Manifest[i].Path == "b.md" {
			plan.Manifest[i].Action = ActionSkip
		}
	}
	if err := SavePlanAtomic(planPath, plan); err != nil {
		t.Fatal(err)
	}

	summary, err := Commit(api, CommitOptions{PlanPath: planPath})
	if err != nil {
		t.Fatal(err)
	}
	if summary.LinksSkippedUnresolvable != 1 {
		t.Fatalf("expected one unresolvable pair: %#v", summary)
	}
	if !summary.Plan.Complete || summary.Plan.RowsRemaining != 0 ||
		summary.Rows.CompletedNow != 1 || summary.Rows.ReviewSkipped != 1 {
		t.Fatalf("unresolvable links must not prevent plan completion: %#v", summary)
	}
}

func proposeFixtureDigest(t *testing.T, sourcePath, adapter string) Digest {
	t.Helper()
	planPath := filepath.Join(t.TempDir(), "import.plan.yaml")
	_, digest, err := Propose(newTestAPI(), ProposeOptions{
		SourcePath: sourcePath, PlanPath: planPath, Workspace: 1, Adapter: adapter,
	})
	if err != nil {
		t.Fatal(err)
	}
	return digest
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
