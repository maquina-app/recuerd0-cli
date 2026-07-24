package importer

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/client"
)

type pagingAPI struct {
	pages       map[string]*client.APIResponse
	getCalls    []string
	postCalls   int
	patchCalls  int
	deleteCalls int
}

func (api *pagingAPI) Get(path string) (*client.APIResponse, error) {
	api.getCalls = append(api.getCalls, path)
	return api.pages[path], nil
}

func (api *pagingAPI) GetWithPagination(path string) (*client.APIResponse, error) {
	api.getCalls = append(api.getCalls, path)
	return api.pages[path], nil
}

func (api *pagingAPI) Post(string, interface{}) (*client.APIResponse, error) {
	api.postCalls++
	return &client.APIResponse{StatusCode: 201}, nil
}

func (api *pagingAPI) Patch(string, interface{}) (*client.APIResponse, error) {
	api.patchCalls++
	return &client.APIResponse{StatusCode: 200}, nil
}

func (api *pagingAPI) Delete(string) (*client.APIResponse, error) {
	api.deleteCalls++
	return &client.APIResponse{StatusCode: 204}, nil
}

func TestProposePaginatesConflictsAndPerformsGETOnlyDetection(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "vault")
	writeTestFile(t, filepath.Join(source, "ambiguous.md"), "# Ambiguous\n\nBody\n")
	writeTestFile(t, filepath.Join(source, "unique.md"), "# Unique\n\nBody\n")
	initialPath := "/workspaces/9/memories"
	api := &pagingAPI{pages: map[string]*client.APIResponse{
		initialPath: {
			StatusCode: 200,
			Data: []interface{}{
				memoryData(memoryRecord{ID: 9, Title: "AMBIGUOUS", Version: 1}),
			},
			LinkNext: "next-page",
		},
		"next-page": {
			StatusCode: 200,
			Data: []interface{}{
				memoryData(memoryRecord{ID: 3, Title: "ambiguous", Version: 1}),
				memoryData(memoryRecord{ID: 7, Title: "uNiQuE", Version: 1}),
			},
		},
	}}
	planPath := filepath.Join(root, "import.plan.yaml")
	plan, _, err := Propose(api, ProposeOptions{
		SourcePath: source, PlanPath: planPath, Workspace: 9,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(api.getCalls, []string{initialPath, "next-page"}) ||
		api.postCalls != 0 || api.patchCalls != 0 || api.deleteCalls != 0 {
		t.Fatalf("propose conflict detection must paginate with GET only: %#v", api)
	}

	rows := make(map[string]PlanRow, len(plan.Manifest))
	for _, row := range plan.Manifest {
		rows[row.Path] = row
	}
	if rows["unique.md"].TargetMemoryID != 7 || rows["unique.md"].Action != ActionSkip {
		t.Fatalf("unique title match mismatch: %#v", rows["unique.md"])
	}
	if rows["ambiguous.md"].TargetMemoryID != 0 || rows["ambiguous.md"].Action != ActionSkip {
		t.Fatalf("ambiguous title match mismatch: %#v", rows["ambiguous.md"])
	}
	exceptions := make(map[string]Exception)
	for _, exception := range plan.Exceptions {
		if exception.Kind == "conflict" {
			exceptions[exception.Path] = exception
		}
	}
	if !reflect.DeepEqual(exceptions["unique.md"].Candidates, []int64{7}) ||
		!reflect.DeepEqual(exceptions["ambiguous.md"].Candidates, []int64{3, 9}) {
		t.Fatalf("conflict candidates were not deterministic: %#v", exceptions)
	}
}

func TestProposeSeedMismatchRequiresFreshReplacement(t *testing.T) {
	root := t.TempDir()
	firstSource := filepath.Join(root, "first")
	secondSource := filepath.Join(root, "second")
	writeTestFile(t, filepath.Join(firstSource, "one.md"), "# First\n")
	writeTestFile(t, filepath.Join(secondSource, "two.md"), "# Second\n")
	planPath := filepath.Join(root, "import.plan.yaml")
	api := newTestAPI()
	if _, _, err := Propose(api, ProposeOptions{
		SourcePath: firstSource, PlanPath: planPath, Workspace: 1,
	}); err != nil {
		t.Fatal(err)
	}

	_, _, err := Propose(api, ProposeOptions{
		SourcePath: secondSource, PlanPath: planPath, Workspace: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "existing plan seed does not match") {
		t.Fatalf("mismatched seed must require --fresh: %v", err)
	}
	replaced, _, err := Propose(api, ProposeOptions{
		SourcePath: secondSource, PlanPath: planPath, Workspace: 1, Fresh: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(replaced.Manifest) != 1 || replaced.Manifest[0].Path != "two.md" ||
		replaced.SourcePath != canonicalPathForTest(t, secondSource) {
		t.Fatalf("--fresh did not replace the seed: %#v", replaced)
	}
}

func TestThreeBodyIdenticalMarkdownFilesKeepOneCreateCandidate(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "a.md"), "# Same\n\nBody\n")
	writeTestFile(t, filepath.Join(root, "m-copy.md"), "---\ntags: [different]\ncategory: decision\n---\n# Same\n\nBody\n")
	writeTestFile(t, filepath.Join(root, "z-copy.md"), "---\ntags: [other]\ncategory: discovery\n---\n# Same\n\nBody\n")
	result, err := scanMarkdown(root, defaultRules())
	if err != nil {
		t.Fatal(err)
	}
	duplicatePaths := make([]string, 0, 2)
	for _, row := range result.Rows {
		if hasException(row.Exceptions, "dupe_exact") {
			duplicatePaths = append(duplicatePaths, row.Path)
		}
	}
	if !reflect.DeepEqual(duplicatePaths, []string{"m-copy.md", "z-copy.md"}) {
		t.Fatalf("body-only duplicate scoping mismatch: %#v", duplicatePaths)
	}
}

func canonicalPathForTest(t *testing.T, path string) string {
	t.Helper()
	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(absolute)
}
