package importer

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var hashPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// LoadPlan parses a plan with KnownFields enabled, retains YAML locations for
// semantic errors, and validates all source-independent invariants.
func LoadPlan(path string) (*Plan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read plan: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}

	var plan Plan
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&plan); err != nil {
		return nil, fmt.Errorf("parse plan: %w", err)
	}
	var extra interface{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, validationf("plan contains multiple YAML documents")
		}
		return nil, fmt.Errorf("parse plan: %w", err)
	}

	if err := validatePlan(&plan, rootNode(&doc), nil); err != nil {
		return nil, err
	}
	return &plan, nil
}

// SavePlanAtomic writes deterministic YAML through a same-directory temporary
// file, syncs it, renames it, and syncs the containing directory.
func SavePlanAtomic(path string, plan *Plan) error {
	if err := validatePlan(plan, nil, nil); err != nil {
		return err
	}
	data, err := yaml.Marshal(plan)
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create plan directory: %w", err)
	}
	temp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create plan temp file: %w", err)
	}
	tempName := temp.Name()
	ok := false
	defer func() {
		_ = temp.Close()
		if !ok {
			_ = os.Remove(tempName)
		}
	}()
	if err := temp.Chmod(0o644); err != nil {
		return fmt.Errorf("chmod plan temp file: %w", err)
	}
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write plan temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync plan temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close plan temp file: %w", err)
	}
	if err := os.Rename(tempName, path); err != nil {
		return fmt.Errorf("replace plan: %w", err)
	}
	ok = true
	if directory, err := os.Open(dir); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

func validatePlan(plan *Plan, root *yaml.Node, ledgerIDs map[string]int64) error {
	var problems []string
	line := func(path ...string) int {
		if root == nil {
			return 0
		}
		node := findYAMLNode(root, path...)
		if node == nil {
			return 0
		}
		return node.Line
	}
	if plan.Format != PlanFormat {
		appendProblem(&problems, line("format"), "format must be %q, got %q", PlanFormat, plan.Format)
	}
	if plan.Version != PlanVersion {
		appendProblem(&problems, line("version"), "version must be %d, got %d", PlanVersion, plan.Version)
	}
	if plan.SourcePath == "" || !filepath.IsAbs(plan.SourcePath) {
		appendProblem(&problems, line("source_path"), "source_path must be absolute")
	}
	if !validAdapter(plan.Adapter) {
		appendProblem(&problems, line("adapter"), "adapter must be %s or %s", AdapterObsidianMarkdown, AdapterWorkspaceExport)
	}
	if plan.Workspace <= 0 {
		appendProblem(&problems, line("workspace"), "workspace must be a positive integer")
	}
	if !validCategories[plan.Rules.DefaultCategory] {
		appendProblem(&problems, line("rules", "default_category"), "invalid default category %q", plan.Rules.DefaultCategory)
	}
	for prefix, category := range plan.Rules.CategoryMap {
		if strings.TrimSpace(prefix) == "" {
			appendProblem(&problems, line("rules", "category_map"), "category_map contains an empty path prefix")
		}
		if !validCategories[category] {
			appendProblem(&problems, line("rules", "category_map", prefix), "invalid mapped category %q", category)
		}
	}

	rows := make(map[string]int, len(plan.Manifest))
	for i, row := range plan.Manifest {
		index := fmt.Sprintf("%d", i)
		if row.Path == "" {
			appendProblem(&problems, line("manifest", index, "path"), "manifest path is empty")
		}
		if previous, exists := rows[row.Path]; exists {
			appendProblem(&problems, line("manifest", index, "path"), "duplicate manifest path %q (first at row %d)", row.Path, previous+1)
		} else {
			rows[row.Path] = i
		}
		if !validCategories[row.Category] {
			appendProblem(&problems, line("manifest", index, "category"), "invalid category %q for %q", row.Category, row.Path)
		}
		if !validAction(row.Action) {
			appendProblem(&problems, line("manifest", index, "action"), "invalid action %q for %q", row.Action, row.Path)
		}
		for name, hash := range map[string]string{
			"content_hash": row.ContentHash, "source_hash": row.SourceHash, "row_fingerprint": row.RowFingerprint,
		} {
			if !hashPattern.MatchString(hash) {
				appendProblem(&problems, line("manifest", index, name), "%s for %q must use sha256:<hex>", name, row.Path)
			}
		}
		if row.Action == ActionCreate && row.TargetMemoryID != 0 {
			appendProblem(&problems, line("manifest", index, "target_memory_id"), "target_memory_id is invalid with create for %q", row.Path)
		}
		if row.TargetMemoryID < 0 {
			appendProblem(&problems, line("manifest", index, "target_memory_id"), "target_memory_id must be positive for %q", row.Path)
		}
		if ledgerID := ledgerIDs[row.Path]; ledgerID != 0 && row.TargetMemoryID != 0 && ledgerID != row.TargetMemoryID {
			appendProblem(&problems, line("manifest", index, "target_memory_id"), "target_memory_id %d disagrees with ledger memory_id %d for %q", row.TargetMemoryID, ledgerID, row.Path)
		}
		if ledgerIDs != nil && row.Action == ActionVersion && row.TargetMemoryID == 0 && ledgerIDs[row.Path] == 0 {
			appendProblem(&problems, line("manifest", index, "action"), "version requires target_memory_id or ledger identity for %q", row.Path)
		}
		if plan.Adapter == AdapterWorkspaceExport {
			expected := fmt.Sprintf("workspace/%d/memories/%d", row.SourceWorkspaceID, row.RootID)
			if row.SourceWorkspaceID <= 0 || row.RootID <= 0 || row.Versions <= 0 || row.Path != expected {
				appendProblem(&problems, line("manifest", index, "path"), "invalid workspace_export identity for %q", row.Path)
			}
			if len(row.Links) != 0 {
				appendProblem(&problems, line("manifest", index, "links"), "workspace_export rows cannot contain links")
			}
		} else {
			if row.SourceWorkspaceID != 0 || row.RootID != 0 || row.Versions != 0 {
				appendProblem(&problems, line("manifest", index, "path"), "export-only identity fields are invalid for %q", row.Path)
			}
			if filepath.IsAbs(row.Path) || strings.HasPrefix(row.Path, "./") || strings.Contains(row.Path, `\`) {
				appendProblem(&problems, line("manifest", index, "path"), "markdown path %q must be source-relative and forward-slashed", row.Path)
			}
		}
		if containsDuplicate(row.Tags) {
			appendProblem(&problems, line("manifest", index, "tags"), "tags for %q must be unique", row.Path)
		}
		if containsDuplicate(row.Links) {
			appendProblem(&problems, line("manifest", index, "links"), "links for %q must be unique", row.Path)
		}
	}

	resolutions := make(map[string][]string)
	exceptionKeys := make(map[string]int)
	for i, exception := range plan.Exceptions {
		index := fmt.Sprintf("%d", i)
		if _, ok := rows[exception.Path]; !ok {
			appendProblem(&problems, line("exceptions", index, "path"), "exception references missing manifest row %q", exception.Path)
		}
		if !validExceptionKind(exception.Kind) {
			appendProblem(&problems, line("exceptions", index, "kind"), "invalid exception kind %q for %q", exception.Kind, exception.Path)
		}
		if !validAction(exception.Resolution) {
			appendProblem(&problems, line("exceptions", index, "resolution"), "invalid resolution %q for %q", exception.Resolution, exception.Path)
		}
		key := exception.Path + "\x00" + exception.Kind
		if previous, exists := exceptionKeys[key]; exists {
			appendProblem(&problems, line("exceptions", index, "kind"), "duplicate exception (%q, %q); first appears at exception %d", exception.Path, exception.Kind, previous+1)
		} else {
			exceptionKeys[key] = i
		}
		resolutions[exception.Path] = append(resolutions[exception.Path],
			fmt.Sprintf("line %d exception %s=%q", line("exceptions", index, "resolution"), exception.Kind, exception.Resolution))
	}
	for path, values := range resolutions {
		rowIndex := rows[path]
		action := plan.Manifest[rowIndex].Action
		var disagreements []string
		for _, value := range values {
			if !strings.HasSuffix(value, "="+fmt.Sprintf("%q", action)) {
				disagreements = append(disagreements, value)
			}
		}
		if len(disagreements) > 0 {
			location := line("manifest", fmt.Sprintf("%d", rowIndex), "action")
			all := append([]string{fmt.Sprintf("line %d manifest action=%q", location, action)}, disagreements...)
			appendProblem(&problems, 0, "action/resolution disagreement for %q: %s", path, strings.Join(all, ", "))
		}
	}
	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

func ValidatePlanWithLedger(plan *Plan, ledger *Ledger) error {
	ids := make(map[string]int64)
	if ledger != nil {
		for path, state := range ledger.Paths {
			ids[path] = state.MemoryID
		}
	}
	return validatePlan(plan, nil, ids)
}

func validatePlanFileWithLedger(path string, plan *Plan, ledger *Ledger) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read plan: %w", err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parse plan: %w", err)
	}
	ids := make(map[string]int64)
	if ledger != nil {
		for rowPath, state := range ledger.Paths {
			ids[rowPath] = state.MemoryID
		}
	}
	return validatePlan(plan, rootNode(&doc), ids)
}

func validAdapter(adapter string) bool {
	return adapter == AdapterObsidianMarkdown || adapter == AdapterWorkspaceExport
}

func validAction(action string) bool {
	return action == ActionCreate || action == ActionVersion || action == ActionSkip
}

func validExceptionKind(kind string) bool {
	switch kind {
	case "conflict", "unparseable", "title_too_long", "guessed_title", "dupe_title", "dupe_exact":
		return true
	default:
		return false
	}
}

func containsDuplicate(values []string) bool {
	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}

func rootNode(doc *yaml.Node) *yaml.Node {
	if doc == nil {
		return nil
	}
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		return doc.Content[0]
	}
	return doc
}

func findYAMLNode(node *yaml.Node, path ...string) *yaml.Node {
	if node == nil || len(path) == 0 {
		return node
	}
	switch node.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == path[0] {
				return findYAMLNode(node.Content[i+1], path[1:]...)
			}
		}
	case yaml.SequenceNode:
		var index int
		if _, err := fmt.Sscanf(path[0], "%d", &index); err == nil && index >= 0 && index < len(node.Content) {
			return findYAMLNode(node.Content[index], path[1:]...)
		}
	}
	return nil
}

func canonicalPlanPath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func defaultLedgerPath(planPath string) string {
	return filepath.Join(filepath.Dir(planPath), "import.ledger.jsonl")
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}
