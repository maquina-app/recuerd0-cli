package importer

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/maquina/recuerd0-cli/internal/client"
)

// Propose scans source material, performs GET-only conflict detection, merges
// review-owned fields from a matching existing plan, and atomically saves it.
func Propose(api client.API, options ProposeOptions) (*Plan, Digest, error) {
	if options.Workspace <= 0 {
		return nil, Digest{}, validationf("workspace must be a positive integer")
	}
	sourcePath, err := canonicalPlanPath(options.SourcePath)
	if err != nil {
		return nil, Digest{}, fmt.Errorf("resolve source path: %w", err)
	}
	planPath := options.PlanPath
	if planPath == "" {
		planPath = "import.plan.yaml"
	}
	planPath, err = canonicalPlanPath(planPath)
	if err != nil {
		return nil, Digest{}, fmt.Errorf("resolve plan path: %w", err)
	}
	ledgerPath := options.LedgerPath
	if ledgerPath == "" {
		ledgerPath = defaultLedgerPath(planPath)
	}
	ledgerPath, err = canonicalPlanPath(ledgerPath)
	if err != nil {
		return nil, Digest{}, fmt.Errorf("resolve ledger path: %w", err)
	}
	if sourcePath == planPath || sourcePath == ledgerPath || planPath == ledgerPath {
		return nil, Digest{}, validationf("source, plan, and ledger paths must be distinct")
	}

	adapter, err := detectAdapter(sourcePath, options.Adapter)
	if err != nil {
		return nil, Digest{}, err
	}
	rules := defaultRules()
	var prior *Plan
	exists, err := fileExists(planPath)
	if err != nil {
		return nil, Digest{}, fmt.Errorf("inspect existing plan: %w", err)
	}
	if exists && !options.Fresh {
		prior, err = LoadPlan(planPath)
		if err != nil {
			return nil, Digest{}, err
		}
		if prior.SourcePath != sourcePath || prior.Adapter != adapter || prior.Workspace != options.Workspace {
			return nil, Digest{}, validationf("existing plan seed does not match absolute source path, adapter, and workspace; use --fresh to replace it")
		}
		rules = prior.Rules
	}

	ledger, err := LoadLedger(ledgerPath)
	if err != nil {
		return nil, Digest{}, err
	}
	for _, record := range ledger.Records {
		if record.Workspace != options.Workspace {
			return nil, Digest{}, validationf("ledger line %d belongs to workspace %d, not %d", record.Line, record.Workspace, options.Workspace)
		}
	}

	scanRules := rulesWithImportArtifactsExcluded(rules, sourcePath, planPath, ledgerPath)
	scanned, err := scanSource(sourcePath, adapter, scanRules)
	if err != nil {
		return nil, Digest{}, err
	}
	plan := &Plan{
		Format: PlanFormat, Version: PlanVersion, SourcePath: sourcePath,
		Adapter: adapter, Workspace: options.Workspace, Rules: rules, Scan: scanned.Stats,
	}
	mergeRows(plan, scanned.Rows, prior)
	classifyLedger(plan, ledger)
	if err := classifyServerConflicts(api, plan, ledger); err != nil {
		return nil, Digest{}, err
	}
	restoreReviewedTargets(plan, prior, ledger)
	restoreExceptionResolutions(plan, prior)
	alignActionsAndExceptions(plan)
	sort.Slice(plan.Manifest, func(i, j int) bool {
		if adapter == AdapterWorkspaceExport {
			return plan.Manifest[i].RootID < plan.Manifest[j].RootID
		}
		return plan.Manifest[i].Path < plan.Manifest[j].Path
	})
	sortExceptions(plan.Exceptions)
	if err := validatePlan(plan, nil, ledger.identityMap()); err != nil {
		return nil, Digest{}, err
	}
	if err := SavePlanAtomic(planPath, plan); err != nil {
		return nil, Digest{}, err
	}
	return plan, PlanDigest(plan), nil
}

func rulesWithImportArtifactsExcluded(rules Rules, sourcePath string, artifactPaths ...string) Rules {
	copy := rules
	copy.Exclude = append([]string(nil), rules.Exclude...)
	copy.ignore = append([]string(nil), rules.ignore...)
	for _, artifactPath := range artifactPaths {
		relative, err := filepath.Rel(sourcePath, artifactPath)
		if err != nil || relative == "." || filepath.IsAbs(relative) ||
			relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		manifestPath := filepath.ToSlash(relative)
		found := false
		for _, existing := range copy.ignore {
			if existing == manifestPath {
				found = true
				break
			}
		}
		if !found {
			copy.ignore = append(copy.ignore, manifestPath)
		}
	}
	return copy
}

func detectAdapter(sourcePath, explicit string) (string, error) {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return "", fmt.Errorf("inspect import source: %w", err)
	}
	if explicit != "" && !validAdapter(explicit) {
		return "", validationf("adapter must be %s or %s", AdapterObsidianMarkdown, AdapterWorkspaceExport)
	}
	if explicit == AdapterObsidianMarkdown || (explicit == "" && info.IsDir()) {
		if !info.IsDir() {
			return "", validationf("obsidian_markdown source must be a directory")
		}
		return AdapterObsidianMarkdown, nil
	}
	if info.IsDir() {
		return "", validationf("workspace_export source must be a JSON file")
	}
	if _, err := scanWorkspaceExport(sourcePath); err != nil {
		return "", err
	}
	return AdapterWorkspaceExport, nil
}

func scanSource(sourcePath, adapter string, rules Rules) (*scanResult, error) {
	if adapter == AdapterObsidianMarkdown {
		return scanMarkdown(sourcePath, rules)
	}
	return scanWorkspaceExport(sourcePath)
}

func mergeRows(plan *Plan, scanned []sourceRow, prior *Plan) {
	priorRows := make(map[string]PlanRow)
	priorExceptions := make(map[string]Exception)
	if prior != nil {
		for _, row := range prior.Manifest {
			priorRows[row.Path] = row
		}
		for _, exception := range prior.Exceptions {
			priorExceptions[exception.Path+"\x00"+exception.Kind] = exception
		}
	}
	for _, source := range scanned {
		row := PlanRow{
			Path: source.Path, Title: source.Title, Category: source.Category,
			Tags: append([]string(nil), source.Tags...), Links: append([]string(nil), source.Links...),
			Action: ActionCreate, SourceHash: source.SourceHash,
			SourceWorkspaceID: source.SourceWorkspaceID, RootID: source.RootID,
			Versions: len(source.Versions), Notes: append([]string(nil), source.Notes...),
		}
		row.RowFingerprint = rowFingerprint(row.Title, row.Category, row.Tags, row.Links)
		if plan.Adapter == AdapterWorkspaceExport {
			latest := source.Versions[len(source.Versions)-1]
			row.ContentHash = CanonicalTupleHash(latest.Title, latest.Tags, latest.Category, latest.Body)
		} else {
			row.ContentHash = CanonicalTupleHash(row.Title, row.Tags, row.Category, source.Body)
		}

		if old, ok := priorRows[source.Path]; ok {
			edited := rowFingerprint(old.Title, old.Category, old.Tags, old.Links) != old.RowFingerprint
			row.Action = old.Action
			row.TargetMemoryID = old.TargetMemoryID
			row.Notes = append([]string(nil), old.Notes...)
			if edited {
				row.Title = old.Title
				row.Category = old.Category
				row.Tags = append([]string(nil), old.Tags...)
				row.Links = append([]string(nil), old.Links...)
				row.RowFingerprint = old.RowFingerprint
				row.Notes = appendNoteOnce(row.Notes, "row edited — rules changes not applied")
				if plan.Adapter == AdapterObsidianMarkdown {
					row.ContentHash = CanonicalTupleHash(row.Title, row.Tags, row.Category, source.Body)
				}
			}
		}
		plan.Manifest = append(plan.Manifest, row)

		for _, exception := range source.Exceptions {
			if old, ok := priorExceptions[exception.Path+"\x00"+exception.Kind]; ok {
				exception.Resolution = old.Resolution
			}
			plan.Exceptions = append(plan.Exceptions, exception)
		}
	}
}

func classifyLedger(plan *Plan, ledger *Ledger) {
	for i := range plan.Manifest {
		row := &plan.Manifest[i]
		state := ledger.Paths[row.Path]
		if state == nil || len(state.Revisions) == 0 {
			continue
		}
		latest := state.Revisions[len(state.Revisions)-1]
		matching := revisionForRow(state, *row)
		if latest.CommittedPrefix < latest.ChainLen {
			if matching == latest {
				if latest.Action != ActionCreate {
					row.TargetMemoryID = state.MemoryID
				} else {
					row.TargetMemoryID = 0
				}
				row.Action = latest.Action
			} else {
				row.Action = ActionSkip
				row.TargetMemoryID = state.MemoryID
				plan.Exceptions = append(plan.Exceptions, Exception{
					Path: row.Path, Kind: "conflict", Resolution: ActionSkip,
					Detail: "different-revision partial chain may only remain skipped",
				})
			}
			continue
		}
		if matching == latest && latest.CommittedPrefix == latest.ChainLen {
			row.Action = ActionSkip
			row.TargetMemoryID = state.MemoryID
			row.Notes = removeNote(row.Notes, "changed since import")
			row.Notes = appendNoteOnce(row.Notes, "already imported, unchanged")
			continue
		}
		if plan.Adapter == AdapterWorkspaceExport {
			row.Action = ActionSkip
			row.TargetMemoryID = state.MemoryID
			plan.Exceptions = append(plan.Exceptions, Exception{
				Path: row.Path, Kind: "conflict", Resolution: ActionSkip,
				Detail: "completed workspace export changed; export deltas are not merged",
			})
		} else {
			row.Action = ActionVersion
			row.TargetMemoryID = state.MemoryID
			row.Notes = removeNote(row.Notes, "already imported, unchanged")
			row.Notes = appendNoteOnce(row.Notes, "changed since import")
		}
	}
}

func restoreExceptionResolutions(plan, prior *Plan) {
	if prior == nil {
		return
	}
	priorResolution := make(map[string]string)
	automaticCompleted := make(map[string]bool)
	for _, row := range prior.Manifest {
		for _, note := range row.Notes {
			if note == "already imported, unchanged" {
				automaticCompleted[row.Path] = true
			}
		}
	}
	for _, exception := range prior.Exceptions {
		priorResolution[exception.Path+"\x00"+exception.Kind] = exception.Resolution
	}
	for i := range plan.Exceptions {
		exception := &plan.Exceptions[i]
		if strings.Contains(exception.Detail, "different-revision partial chain") ||
			strings.Contains(exception.Detail, "completed workspace export changed") ||
			automaticCompleted[exception.Path] {
			continue
		}
		if resolution, ok := priorResolution[exception.Path+"\x00"+exception.Kind]; ok {
			exception.Resolution = resolution
		}
	}
}

func restoreReviewedTargets(plan, prior *Plan, ledger *Ledger) {
	if prior == nil {
		return
	}
	priorTargets := make(map[string]int64, len(prior.Manifest))
	for _, row := range prior.Manifest {
		priorTargets[row.Path] = row.TargetMemoryID
	}
	for i := range plan.Manifest {
		row := &plan.Manifest[i]
		if ledger.Paths[row.Path] != nil {
			continue
		}
		if target, ok := priorTargets[row.Path]; ok {
			row.TargetMemoryID = target
		}
	}
}

func classifyServerConflicts(api client.API, plan *Plan, ledger *Ledger) error {
	needsListing := false
	for _, row := range plan.Manifest {
		if ledger.Paths[row.Path] == nil {
			needsListing = true
			break
		}
	}
	if !needsListing {
		return nil
	}
	memories, err := listWorkspaceMemories(api, plan.Workspace, "", "")
	if err != nil {
		return fmt.Errorf("detect import conflicts: %w", err)
	}
	byTitle := make(map[string][]memoryRecord)
	for _, memory := range memories {
		byTitle[strings.ToLower(memory.Title)] = append(byTitle[strings.ToLower(memory.Title)], memory)
	}
	for i := range plan.Manifest {
		row := &plan.Manifest[i]
		if ledger.Paths[row.Path] != nil {
			continue
		}
		matches := byTitle[strings.ToLower(row.Title)]
		if len(matches) == 0 {
			continue
		}
		sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
		candidates := make([]int64, len(matches))
		for i := range matches {
			candidates[i] = matches[i].ID
		}
		exception := Exception{
			Path: row.Path, Kind: "conflict", Resolution: ActionSkip,
			Candidates: candidates,
		}
		if len(matches) == 1 {
			row.TargetMemoryID = matches[0].ID
			exception.Detail = fmt.Sprintf("case-insensitive title match: memory %d", matches[0].ID)
		} else {
			row.TargetMemoryID = 0
			exception.Detail = "ambiguous case-insensitive title matches"
		}
		plan.Exceptions = append(plan.Exceptions, exception)
	}
	return nil
}

func alignActionsAndExceptions(plan *Plan) {
	exceptionsByPath := make(map[string][]int)
	for i := range plan.Exceptions {
		exceptionsByPath[plan.Exceptions[i].Path] = append(exceptionsByPath[plan.Exceptions[i].Path], i)
	}
	for i := range plan.Manifest {
		row := &plan.Manifest[i]
		merged := row.Action
		for _, index := range exceptionsByPath[row.Path] {
			merged = strongerAction(merged, plan.Exceptions[index].Resolution)
		}
		row.Action = merged
		for _, index := range exceptionsByPath[row.Path] {
			plan.Exceptions[index].Resolution = merged
		}
	}
}

func strongerAction(first, second string) string {
	rank := map[string]int{ActionCreate: 1, ActionVersion: 2, ActionSkip: 3}
	if rank[second] > rank[first] {
		return second
	}
	return first
}

func appendNoteOnce(notes []string, note string) []string {
	for _, existing := range notes {
		if existing == note {
			return notes
		}
	}
	return append(notes, note)
}

func removeNote(notes []string, unwanted string) []string {
	result := notes[:0]
	for _, note := range notes {
		if note != unwanted {
			result = append(result, note)
		}
	}
	return result
}

type memoryRecord struct {
	ID       int64
	Title    string
	Body     string
	Tags     []string
	Category string
	Source   string
	Version  int
}

func listWorkspaceMemories(api client.API, workspace int64, title, source string) ([]memoryRecord, error) {
	path := fmt.Sprintf("/workspaces/%d/memories", workspace)
	values := url.Values{}
	if title != "" {
		values.Set("title", title)
	}
	if source != "" {
		values.Set("source", source)
	}
	if encoded := values.Encode(); encoded != "" {
		path += "?" + encoded
	}
	var result []memoryRecord
	for path != "" {
		response, err := api.GetWithPagination(path)
		if err != nil {
			return nil, err
		}
		for _, item := range responseItems(response.Data) {
			memory, err := decodeMemory(item)
			if err == nil && memory.ID > 0 {
				result = append(result, memory)
			}
		}
		path = response.LinkNext
	}
	return result, nil
}

func responseItems(data interface{}) []interface{} {
	switch typed := data.(type) {
	case []interface{}:
		return typed
	case []map[string]interface{}:
		result := make([]interface{}, len(typed))
		for i := range typed {
			result[i] = typed[i]
		}
		return result
	case map[string]interface{}:
		for _, key := range []string{"memories", "data", "items"} {
			if array, ok := typed[key].([]interface{}); ok {
				return array
			}
		}
	}
	return nil
}

func decodeMemory(value interface{}) (memoryRecord, error) {
	object, ok := value.(map[string]interface{})
	if !ok {
		return memoryRecord{}, fmt.Errorf("memory response is not an object")
	}
	id := firstInt64(object, "root_id", "memory_id", "id")
	title, _ := object["title"].(string)
	source, _ := object["source"].(string)
	category, _ := object["category"].(string)
	if category == "" {
		category = "general"
	}
	version := firstInt(object, "version")
	body := ""
	switch content := object["content"].(type) {
	case string:
		body = content
	case map[string]interface{}:
		body, _ = content["body"].(string)
	}
	if body == "" {
		body, _ = object["body"].(string)
	}
	tags, _ := stringSlice(object["tags"])
	return memoryRecord{ID: id, Title: title, Body: body, Tags: tags, Category: category, Source: source, Version: version}, nil
}

func parseIDFromLocation(location string) int64 {
	location = strings.TrimSuffix(strings.SplitN(location, "?", 2)[0], "/")
	part := filepath.Base(location)
	part = strings.TrimSuffix(part, ".json")
	id, _ := strconv.ParseInt(part, 10, 64)
	return id
}
