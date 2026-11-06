package importer

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/maquina/recuerd0-cli/internal/client"
	clierrors "github.com/maquina/recuerd0-cli/internal/errors"
)

type preparedImport struct {
	plan       *Plan
	ledger     *Ledger
	ledgerPath string
	sources    map[string]sourceRow
}

type rowPreflight struct {
	chainBase int
	memoryID  int64
	prior     *memoryRecord
}

// Review validates the plan, ledger, agreement, export fidelity, and source
// hashes without performing network writes.
func Review(options CommitOptions) (*Plan, Digest, string, error) {
	prepared, err := prepareImport(options)
	if err != nil {
		return nil, Digest{}, "", err
	}
	digest := PlanDigest(prepared.plan)
	digest.LeftOut = leftOutRows(prepared.plan, prepared.ledger)
	return prepared.plan, digest, prepared.ledgerPath, nil
}

// Commit executes write rows sequentially, recording an fsynced intent before
// every pass-1 POST and verifying each resulting version through a fresh GET.
func Commit(api client.API, options CommitOptions) (CommitSummary, error) {
	prepared, err := prepareImport(options)
	if err != nil {
		summary := summaryForPreparationError(options)
		summary.Aborted = &Abort{Reason: err.Error()}
		return summary, err
	}
	runner := &commitRunner{
		api: api, prepared: prepared,
		initialComplete: make(map[string]bool),
		preflight:       make(map[string]rowPreflight),
	}
	runner.captureInitialState()
	if err := runner.preflightAll(); err != nil {
		var operation *operationError
		if errors.As(err, &operation) {
			runner.abort(operation.Path, operation.Ordinal, operation.Err)
		} else {
			runner.abort("", 0, err)
		}
		runner.refreshSummary()
		return runner.summary, err
	}
	for i := range prepared.plan.Manifest {
		row := &prepared.plan.Manifest[i]
		if row.Action == ActionSkip {
			continue
		}
		if err := runner.executeRow(row); err != nil {
			if runner.summary.Aborted == nil {
				runner.abort(row.Path, nextOrdinal(prepared.ledger, *row), err)
			}
			runner.refreshSummary()
			return runner.summary, err
		}
	}
	runner.ensureLinks()
	runner.refreshSummary()
	if left := leftOutRows(prepared.plan, prepared.ledger); len(left) > 0 {
		return runner.summary, clierrors.NewValidationError(LeftOutMessage(left))
	}
	return runner.summary, nil
}

// leftOutRows are skipped because an earlier attempt may have written part of a
// different revision of them, not because anyone chose to leave them out.
func leftOutRows(plan *Plan, ledger *Ledger) []string {
	var paths []string
	for _, row := range plan.Manifest {
		state := ledger.Paths[row.Path]
		latest := state.lastLanded()
		if row.Action == ActionSkip && latest != nil &&
			latest.CommittedPrefix < latest.ChainLen && revisionForRow(state, row) != latest {
			paths = append(paths, row.Path)
		}
	}
	return paths
}

// LeftOutMessage says what commit couldn't import and how to get it in, because
// the rest of the plan can finish and these still be missing.
func LeftOutMessage(paths []string) string {
	return fmt.Sprintf(
		"%d row(s) were not imported because an earlier attempt may have written part of them: %s. "+
			"Delete any memory it left, then run propose with a new --ledger path.",
		len(paths), strings.Join(paths, ", "),
	)
}

func prepareImport(options CommitOptions) (*preparedImport, error) {
	planPath := options.PlanPath
	if planPath == "" {
		return nil, validationf("plan path is required")
	}
	absolutePlan, err := canonicalPlanPath(planPath)
	if err != nil {
		return nil, err
	}
	plan, err := LoadPlan(absolutePlan)
	if err != nil {
		return nil, err
	}
	ledgerPath := options.LedgerPath
	if ledgerPath == "" {
		ledgerPath = defaultLedgerPath(absolutePlan)
	}
	ledgerPath, err = canonicalPlanPath(ledgerPath)
	if err != nil {
		return nil, err
	}
	ledger, err := LoadLedger(ledgerPath)
	if err != nil {
		return nil, err
	}
	for _, record := range ledger.Records {
		if record.Workspace != plan.Workspace {
			return nil, validationf("ledger line %d belongs to workspace %d, not %d", record.Line, record.Workspace, plan.Workspace)
		}
	}
	if err := validatePlanFileWithLedger(absolutePlan, plan, ledger); err != nil {
		return nil, err
	}
	scanRules := rulesWithImportArtifactsExcluded(plan.Rules, plan.SourcePath, absolutePlan, ledgerPath)
	scanned, err := scanSource(plan.SourcePath, plan.Adapter, scanRules)
	if err != nil {
		return nil, err
	}
	sources := make(map[string]sourceRow, len(scanned.Rows))
	for _, row := range scanned.Rows {
		sources[row.Path] = row
	}
	if err := validateSourceFidelity(plan, sources); err != nil {
		return nil, err
	}
	if err := validateLedgerActions(plan, ledger); err != nil {
		return nil, err
	}
	return &preparedImport{plan: plan, ledger: ledger, ledgerPath: ledgerPath, sources: sources}, nil
}

func validateSourceFidelity(plan *Plan, sources map[string]sourceRow) error {
	if len(plan.Manifest) != len(sources) {
		return validationf("source changed since propose — re-run propose")
	}
	for _, row := range plan.Manifest {
		source, ok := sources[row.Path]
		if !ok || source.SourceHash != row.SourceHash {
			return validationf("%s: source changed since propose — re-run propose", row.Path)
		}
		if plan.Adapter == AdapterWorkspaceExport {
			if source.SourceWorkspaceID != row.SourceWorkspaceID ||
				source.RootID != row.RootID || len(source.Versions) != row.Versions {
				return validationf("%s: workspace export identity or version chain changed since propose — re-run propose", row.Path)
			}
			latest := source.Versions[len(source.Versions)-1]
			if row.Title != latest.Title || row.Category != latest.Category ||
				!equalStringSlices(row.Tags, latest.Tags) || len(row.Links) != 0 ||
				row.ContentHash != CanonicalTupleHash(latest.Title, latest.Tags, latest.Category, latest.Body) {
				return validationf("%s: export plan metadata or versions disagree with source", row.Path)
			}
		} else {
			expected := CanonicalTupleHash(row.Title, row.Tags, row.Category, source.Body)
			if row.ContentHash != expected {
				return validationf("%s: content_hash disagrees with reviewed metadata or source — re-run propose", row.Path)
			}
		}
	}
	return nil
}

func validateLedgerActions(plan *Plan, ledger *Ledger) error {
	var problems []string
	for _, row := range plan.Manifest {
		state := ledger.Paths[row.Path]
		latest := state.lastLanded()
		if latest == nil {
			continue
		}
		current := revisionForRow(state, row)
		if latest.CommittedPrefix < latest.ChainLen {
			if current == latest {
				// A refusal settles what the server holds, so stopping here is safe. After
				// a lost response it isn't known yet: the next commit finds out.
				settled := row.Action == ActionSkip && latest.UnmatchedIntent == nil
				if row.Action != latest.Action && !settled {
					problems = append(problems, fmt.Sprintf("%s: an earlier attempt may have landed, so run commit again to settle it before changing this row (it must resume action %q, got %q)", row.Path, latest.Action, row.Action))
				}
			} else if row.Action != ActionSkip {
				problems = append(problems, fmt.Sprintf("%s: a different-revision partial chain may only remain skipped", row.Path))
			}
		}
	}
	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

type commitRunner struct {
	api             client.API
	prepared        *preparedImport
	preflight       map[string]rowPreflight
	initialComplete map[string]bool
	summary         CommitSummary
}

type operationError struct {
	Path    string
	Ordinal int
	Err     error
}

func (err *operationError) Error() string {
	return fmt.Sprintf("%s ordinal %d: %v", err.Path, err.Ordinal, err.Err)
}

func (err *operationError) Unwrap() error {
	return err.Err
}

func (runner *commitRunner) captureInitialState() {
	for _, row := range runner.prepared.plan.Manifest {
		runner.initialComplete[row.Path] = rowRevisionComplete(runner.prepared.ledger, row)
	}
	runner.summary = emptyCommitSummary(CommitOptions{LedgerPath: runner.prepared.ledgerPath})
	runner.summary.LedgerPath = runner.prepared.ledgerPath
}

func emptyCommitSummary(options CommitOptions) CommitSummary {
	ledgerPath := options.LedgerPath
	if ledgerPath == "" && options.PlanPath != "" {
		ledgerPath = defaultLedgerPath(options.PlanPath)
	}
	return CommitSummary{LinksFailed: []LinkFailure{}, LedgerPath: ledgerPath}
}

func summaryForPreparationError(options CommitOptions) CommitSummary {
	summary := emptyCommitSummary(options)
	absolutePlan, err := canonicalPlanPath(options.PlanPath)
	if err != nil {
		return summary
	}
	plan, err := LoadPlan(absolutePlan)
	if err != nil {
		return summary
	}
	ledgerPath := options.LedgerPath
	if ledgerPath == "" {
		ledgerPath = defaultLedgerPath(absolutePlan)
	}
	if absoluteLedger, pathErr := canonicalPlanPath(ledgerPath); pathErr == nil {
		ledgerPath = absoluteLedger
	}
	summary.LedgerPath = ledgerPath
	ledger, _ := LoadLedger(ledgerPath)
	if ledger == nil {
		ledger = &Ledger{Paths: make(map[string]*PathLedgerState)}
	}
	summary.Plan.RowsTotal = len(plan.Manifest)
	for _, row := range plan.Manifest {
		if rowRevisionComplete(ledger, row) {
			summary.Plan.RowsCommitted++
			summary.Rows.AlreadyCommitted++
		} else if row.Action == ActionSkip {
			summary.Plan.RowsReviewSkip++
			summary.Rows.ReviewSkipped++
		} else {
			summary.Plan.RowsRemaining++
		}
	}
	return summary
}

func (runner *commitRunner) preflightAll() error {
	memoryCache := make(map[int64]memoryRecord)
	identityOwner := make(map[int64]string)
	claimIdentity := func(memoryID int64, path string) error {
		if previous := identityOwner[memoryID]; previous != "" && previous != path {
			return fmt.Errorf("target memory %d is also used by write row %s", memoryID, previous)
		}
		identityOwner[memoryID] = path
		return nil
	}
	for _, row := range runner.prepared.plan.Manifest {
		if row.Action == ActionSkip || rowRevisionComplete(runner.prepared.ledger, row) {
			continue
		}
		ordinal := nextOrdinal(runner.prepared.ledger, row)
		fail := func(err error) error {
			return &operationError{Path: row.Path, Ordinal: ordinal, Err: err}
		}
		state := runner.prepared.ledger.Paths[row.Path]
		revision := liveRevision(state, row)
		if revision == nil {
			if row.Action == ActionCreate {
				runner.preflight[row.Path] = rowPreflight{chainBase: 0}
				continue
			}
			memoryID := row.TargetMemoryID
			if state != nil && state.MemoryID != 0 {
				memoryID = state.MemoryID
			}
			if memoryID <= 0 {
				return fail(fmt.Errorf("version has no target memory identity"))
			}
			if err := claimIdentity(memoryID, row.Path); err != nil {
				return fail(err)
			}
			memory, ok := memoryCache[memoryID]
			if !ok {
				var err error
				memory, err = runner.getMemory(memoryID)
				if err != nil {
					return fail(fmt.Errorf("preflight target %d: %w", memoryID, err))
				}
				memoryCache[memoryID] = memory
			}
			// The server refuses both, and this is the last moment to say so before
			// anything has been written.
			first := sourceVersion(runner.prepared.plan.Adapter, runner.prepared.sources[row.Path], row, 1)
			if memoryTupleEqual(memory, first) {
				return fail(fmt.Errorf("already matches memory %d, so a new version would change nothing; set this row to skip", memoryID))
			}
			if blank(first.Body) && !blank(memory.Body) {
				return fail(fmt.Errorf("has no content and would overwrite memory %d; set this row to skip", memoryID))
			}
			copy := memory
			runner.preflight[row.Path] = rowPreflight{chainBase: memory.Version, memoryID: memoryID, prior: &copy}
			continue
		}

		memoryID := revision.MemoryID
		if memoryID == 0 && state != nil {
			memoryID = state.MemoryID
		}
		if memoryID == 0 && revision.UnmatchedIntent != nil && revision.UnmatchedIntent.Ordinal == 1 && revision.Action == ActionCreate {
			// The create response may have been lost; title/source reconciliation
			// will resolve the immutable identity during executeRow.
			runner.preflight[row.Path] = rowPreflight{chainBase: revision.ChainBase}
			continue
		}
		if memoryID <= 0 {
			return fail(fmt.Errorf("ledger revision has no fixed memory identity"))
		}
		if err := claimIdentity(memoryID, row.Path); err != nil {
			return fail(err)
		}
		memory, ok := memoryCache[memoryID]
		if !ok {
			var err error
			memory, err = runner.getMemory(memoryID)
			if err != nil {
				return fail(fmt.Errorf("resume preflight target %d: %w", memoryID, err))
			}
			memoryCache[memoryID] = memory
		}
		lastVersion := revision.ChainBase
		if revision.LastCommitted != nil {
			lastVersion = revision.LastCommitted.Version
		}
		if revision.UnmatchedIntent != nil {
			expected := revision.UnmatchedIntent.ExpectedVersion
			if memory.Version != lastVersion && memory.Version != expected {
				return fail(fmt.Errorf("concurrent advancement; server version %d, expected %d or %d", memory.Version, lastVersion, expected))
			}
		} else if memory.Version != lastVersion {
			return fail(fmt.Errorf("concurrent advancement; server version %d, expected %d", memory.Version, lastVersion))
		}
		next := revision.CommittedPrefix + 1
		if next <= revision.ChainLen && lastVersion+1 != revision.ChainBase+next {
			return fail(fmt.Errorf("resume arithmetic disagrees (%d + 1 != %d + %d)", lastVersion, revision.ChainBase, next))
		}
		copy := memory
		runner.preflight[row.Path] = rowPreflight{chainBase: revision.ChainBase, memoryID: memoryID, prior: &copy}
	}
	return nil
}

func (runner *commitRunner) executeRow(row *PlanRow) error {
	for {
		state := runner.prepared.ledger.Paths[row.Path]
		revision := liveRevision(state, *row)
		if revision != nil && revision.CommittedPrefix == revision.ChainLen {
			return nil
		}
		if revision != nil && revision.UnmatchedIntent != nil {
			if err := runner.resumeIntent(row, *revision.UnmatchedIntent); err != nil {
				return runner.stop(row, *revision.UnmatchedIntent, err)
			}
			continue
		}
		ordinal := 1
		chainLen := rowChainLen(runner.prepared.plan.Adapter, runner.prepared.sources[row.Path])
		base := runner.preflight[row.Path].chainBase
		memoryID := runner.preflight[row.Path].memoryID
		if revision != nil {
			ordinal = revision.CommittedPrefix + 1
			chainLen = revision.ChainLen
			base = revision.ChainBase
			if revision.MemoryID != 0 {
				memoryID = revision.MemoryID
			}
		} else if state != nil && state.MemoryID != 0 {
			memoryID = state.MemoryID
		}
		version := sourceVersion(runner.prepared.plan.Adapter, runner.prepared.sources[row.Path], *row, ordinal)
		contentHash := CanonicalTupleHash(version.Title, version.Tags, version.Category, version.Body)
		intent := newIntent(row.Path, runner.prepared.plan.Workspace, row.Action,
			ordinal, chainLen, row.SourceHash, contentHash, base+ordinal, base, memoryID)
		if err := AppendLedgerRecord(runner.prepared.ledgerPath, intent); err != nil {
			return err
		}
		if err := runner.reloadLedger(); err != nil {
			return err
		}
		if err := runner.performIntent(row, intent, version, false); err != nil {
			return runner.stop(row, intent, err)
		}
	}
}

// stop ends the run at this row, and writes down a refusal so a later attempt knows
// nothing landed.
func (runner *commitRunner) stop(row *PlanRow, intent LedgerRecord, err error) error {
	runner.abort(row.Path, intent.Ordinal, err)
	var refused *refusedError
	if !errors.As(err, &refused) {
		return err
	}
	if ledgerErr := AppendLedgerRecord(runner.prepared.ledgerPath, newRefused(intent)); ledgerErr != nil {
		return ledgerErr
	}
	if ledgerErr := runner.reloadLedger(); ledgerErr != nil {
		return ledgerErr
	}
	if httpStatus(err) == 422 {
		runner.summary.Aborted.Next = "Fix this and run propose and commit again, or set its action and resolutions to skip and run commit again."
	}
	return err
}

func (runner *commitRunner) resumeIntent(row *PlanRow, intent LedgerRecord) error {
	version := sourceVersion(runner.prepared.plan.Adapter, runner.prepared.sources[row.Path], *row, intent.Ordinal)
	return runner.performIntent(row, intent, version, true)
}

type reconcileState int

const (
	reconcilePrior reconcileState = iota
	reconcileLanded
	reconcileUnexpected
	reconcileAmbiguous
)

func (runner *commitRunner) performIntent(row *PlanRow, intent LedgerRecord, version VersionData, dangling bool) error {
	memoryID := intent.MemoryID
	if memoryID == 0 {
		memoryID = runner.preflight[row.Path].memoryID
	}
	if dangling {
		state, resolvedID, err := runner.reconcile(intent, version, memoryID)
		if err != nil {
			return err
		}
		switch state {
		case reconcileLanded:
			return runner.recordCommitted(intent, resolvedID, true)
		case reconcileUnexpected:
			return fmt.Errorf("server state does not match the prior or intended tuple/version")
		case reconcileAmbiguous:
			return fmt.Errorf("multiple exact create candidates make the intent ambiguous")
		}
		if err := runner.appendRetryIntent(intent); err != nil {
			return err
		}
	}

	response, err := runner.postVersion(intent, version, memoryID)
	if err == nil {
		if createsRoot(intent) {
			memoryID = responseMemoryID(response)
			if memoryID == 0 {
				state, resolvedID, reconcileErr := runner.reconcile(intent, version, 0)
				if reconcileErr != nil {
					return reconcileErr
				}
				if state != reconcileLanded {
					return fmt.Errorf("create response omitted a stable memory id and reconciliation did not find one")
				}
				memoryID = resolvedID
			}
		}
		readBack, getErr := runner.getMemory(memoryID)
		if getErr != nil {
			return fmt.Errorf("read back memory %d: %w", memoryID, getErr)
		}
		if err := verifyMemory(readBack, version, intent.ExpectedVersion); err != nil {
			return err
		}
		return runner.recordCommitted(intent, memoryID, false)
	}
	if !retryable(err) {
		return asRefusal(err)
	}

	state, resolvedID, reconcileErr := runner.reconcile(intent, version, memoryID)
	if reconcileErr != nil {
		return reconcileErr
	}
	switch state {
	case reconcileLanded:
		return runner.recordCommitted(intent, resolvedID, true)
	case reconcileUnexpected:
		return fmt.Errorf("write outcome is ambiguous: server has an unexpected tuple or version")
	case reconcileAmbiguous:
		return fmt.Errorf("write outcome is ambiguous: multiple exact create candidates")
	}

	if err := runner.appendRetryIntent(intent); err != nil {
		return err
	}
	response, retryErr := runner.postVersion(intent, version, memoryID)
	if retryErr == nil {
		if createsRoot(intent) {
			memoryID = responseMemoryID(response)
		}
		if memoryID > 0 {
			readBack, getErr := runner.getMemory(memoryID)
			if getErr == nil {
				if verifyErr := verifyMemory(readBack, version, intent.ExpectedVersion); verifyErr == nil {
					return runner.recordCommitted(intent, memoryID, false)
				}
			}
		}
	}
	if retryErr != nil && !retryable(retryErr) {
		return asRefusal(retryErr)
	}
	state, resolvedID, reconcileErr = runner.reconcile(intent, version, memoryID)
	if reconcileErr != nil {
		return reconcileErr
	}
	if state == reconcileLanded {
		return runner.recordCommitted(intent, resolvedID, true)
	}
	if state == reconcileAmbiguous {
		return fmt.Errorf("write outcome remains ambiguous: multiple exact create candidates")
	}
	if state == reconcileUnexpected {
		return fmt.Errorf("write outcome remains ambiguous: unexpected tuple or concurrent version")
	}
	if retryErr != nil {
		return fmt.Errorf("write retry exhausted: %w", retryErr)
	}
	return fmt.Errorf("write retry exhausted without the intended version appearing")
}

// refusedError is the server answering a write with a client error, so nothing
// landed. It only ever wraps the POST's own answer: a 404 reading back a write that
// succeeded says nothing of the kind.
type refusedError struct {
	err error
}

func (err *refusedError) Error() string { return err.err.Error() }
func (err *refusedError) Unwrap() error { return err.err }

func asRefusal(err error) error {
	status := httpStatus(err)
	if status >= 400 && status < 500 && status != 429 {
		return &refusedError{err: err}
	}
	return err
}

func (runner *commitRunner) postVersion(intent LedgerRecord, version VersionData, memoryID int64) (*client.APIResponse, error) {
	if createsRoot(intent) {
		body := map[string]interface{}{"memory": map[string]interface{}{
			"title": version.Title, "content": version.Body, "tags": version.Tags,
			"category": version.Category, "source": "import:" + runner.prepared.plan.Adapter,
		}}
		return runner.api.Post(fmt.Sprintf("/workspaces/%d/memories", runner.prepared.plan.Workspace), body)
	}
	if memoryID <= 0 {
		return nil, fmt.Errorf("ordinal %d has no fixed memory identity", intent.Ordinal)
	}
	body := map[string]interface{}{"version": map[string]interface{}{
		"title": version.Title, "content": version.Body, "tags": version.Tags,
		"category": version.Category,
	}}
	return runner.api.Post(fmt.Sprintf("/workspaces/%d/memories/%d/versions", runner.prepared.plan.Workspace, memoryID), body)
}

func createsRoot(intent LedgerRecord) bool {
	return intent.Action == ActionCreate && intent.Ordinal == 1
}

func (runner *commitRunner) reconcile(intent LedgerRecord, version VersionData, memoryID int64) (reconcileState, int64, error) {
	if createsRoot(intent) {
		source := "import:" + runner.prepared.plan.Adapter
		claimed := make(map[int64]bool)
		for path, state := range runner.prepared.ledger.Paths {
			if path == intent.Path || state == nil {
				continue
			}
			if state.MemoryID != 0 {
				claimed[state.MemoryID] = true
			}
			for _, revision := range state.Revisions {
				if revision != nil && revision.MemoryID != 0 {
					claimed[revision.MemoryID] = true
				}
			}
		}
		candidates, err := listWorkspaceMemories(runner.api, runner.prepared.plan.Workspace, version.Title, source)
		if err != nil {
			return reconcileUnexpected, 0, err
		}
		var matches []int64
		unexpectedCandidate := false
		for _, candidate := range candidates {
			if claimed[candidate.ID] {
				continue
			}
			if !strings.EqualFold(candidate.Title, version.Title) || candidate.Source != source {
				continue
			}
			detail, err := runner.getMemory(candidate.ID)
			if err != nil {
				return reconcileUnexpected, 0, err
			}
			if detail.Source == source && detail.Version == intent.ExpectedVersion && memoryTupleEqual(detail, version) {
				matches = append(matches, detail.ID)
			} else if detail.Source == source && strings.EqualFold(detail.Title, version.Title) {
				unexpectedCandidate = true
			}
		}
		sort.Slice(matches, func(i, j int) bool { return matches[i] < matches[j] })
		if len(matches) == 1 {
			return reconcileLanded, matches[0], nil
		}
		if len(matches) > 1 {
			return reconcileAmbiguous, 0, nil
		}
		if unexpectedCandidate {
			return reconcileUnexpected, 0, nil
		}
		return reconcilePrior, 0, nil
	}
	if memoryID <= 0 {
		return reconcileUnexpected, 0, fmt.Errorf("version reconciliation has no fixed memory identity")
	}
	memory, err := runner.getMemory(memoryID)
	if err != nil {
		return reconcileUnexpected, 0, err
	}
	if memory.Version == intent.ExpectedVersion {
		if memoryTupleEqual(memory, version) {
			return reconcileLanded, memoryID, nil
		}
		return reconcileUnexpected, memoryID, nil
	}
	if memory.Version == intent.ExpectedVersion-1 {
		if runner.priorTupleMatches(intent, memory) {
			return reconcilePrior, memoryID, nil
		}
		return reconcileUnexpected, memoryID, nil
	}
	return reconcileUnexpected, memoryID, nil
}

func (runner *commitRunner) appendRetryIntent(intent LedgerRecord) error {
	retryIntent := intent
	retryIntent.Line = 0
	retryIntent.AttemptedAt = time.Now().UTC()
	if err := AppendLedgerRecord(runner.prepared.ledgerPath, retryIntent); err != nil {
		return err
	}
	return runner.reloadLedger()
}

func (runner *commitRunner) priorTupleMatches(intent LedgerRecord, memory memoryRecord) bool {
	if intent.Ordinal > 1 {
		row := findPlanRow(runner.prepared.plan, intent.Path)
		source := runner.prepared.sources[intent.Path]
		if row == nil {
			return false
		}
		previous := sourceVersion(runner.prepared.plan.Adapter, source, *row, intent.Ordinal-1)
		return memoryTupleEqual(memory, previous)
	}
	preflight := runner.preflight[intent.Path]
	if preflight.prior == nil {
		return false
	}
	return memory.Title == preflight.prior.Title &&
		memory.Body == preflight.prior.Body &&
		equalStringSlices(memory.Tags, preflight.prior.Tags) &&
		memory.Category == preflight.prior.Category &&
		memory.Version == preflight.prior.Version
}

func findPlanRow(plan *Plan, path string) *PlanRow {
	for i := range plan.Manifest {
		if plan.Manifest[i].Path == path {
			return &plan.Manifest[i]
		}
	}
	return nil
}

func (runner *commitRunner) recordCommitted(intent LedgerRecord, memoryID int64, reconciled bool) error {
	if memoryID <= 0 {
		return fmt.Errorf("cannot commit ledger record without a stable memory identity")
	}
	record := newCommitted(intent, memoryID, reconciled)
	if err := AppendLedgerRecord(runner.prepared.ledgerPath, record); err != nil {
		return err
	}
	if createsRoot(intent) {
		runner.summary.Ops.Created++
	} else {
		runner.summary.Ops.Versioned++
	}
	if reconciled {
		runner.summary.Ops.Reconciled++
	}
	return runner.reloadLedger()
}

func (runner *commitRunner) reloadLedger() error {
	ledger, err := LoadLedger(runner.prepared.ledgerPath)
	if err != nil {
		return err
	}
	runner.prepared.ledger = ledger
	return nil
}

func (runner *commitRunner) getMemory(memoryID int64) (memoryRecord, error) {
	path := fmt.Sprintf("/workspaces/%d/memories/%d.json", runner.prepared.plan.Workspace, memoryID)
	response, err := runner.api.Get(path)
	if err != nil && retryable(err) {
		response, err = runner.api.Get(path)
	}
	if err != nil {
		return memoryRecord{}, err
	}
	memory, err := decodeMemory(response.Data)
	if err != nil {
		return memoryRecord{}, err
	}
	if memory.ID == 0 {
		memory.ID = memoryID
	}
	return memory, nil
}

func verifyMemory(memory memoryRecord, intended VersionData, expectedVersion int) error {
	if memory.Version != expectedVersion {
		return fmt.Errorf("read-back version mismatch: expected %d, got %d", expectedVersion, memory.Version)
	}
	if memory.Title != intended.Title {
		return fmt.Errorf("read-back title mismatch")
	}
	if storedBody(memory.Body) != storedBody(intended.Body) {
		return fmt.Errorf("read-back body mismatch")
	}
	if !equalStringSlices(distinctTags(memory.Tags), distinctTags(intended.Tags)) {
		return fmt.Errorf("read-back tag order mismatch")
	}
	if memory.Category != intended.Category {
		return fmt.Errorf("read-back category mismatch")
	}
	return nil
}

func memoryTupleEqual(memory memoryRecord, intended VersionData) bool {
	return sameVersion(VersionData{Title: memory.Title, Body: memory.Body, Tags: memory.Tags, Category: memory.Category}, intended)
}

func equalStringSlices(first, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for i := range first {
		if first[i] != second[i] {
			return false
		}
	}
	return true
}

func responseMemoryID(response *client.APIResponse) int64 {
	if response == nil {
		return 0
	}
	if memory, err := decodeMemory(response.Data); err == nil && memory.ID > 0 {
		return memory.ID
	}
	return parseIDFromLocation(response.Location)
}

func retryable(err error) bool {
	var cliError *clierrors.CLIError
	if errors.As(err, &cliError) {
		return cliError.Code == clierrors.CodeNetwork || cliError.Status >= 500
	}
	return false
}

func rowChainLen(adapter string, source sourceRow) int {
	if adapter == AdapterWorkspaceExport {
		return len(source.Versions)
	}
	return 1
}

func sourceVersion(adapter string, source sourceRow, row PlanRow, ordinal int) VersionData {
	if adapter == AdapterWorkspaceExport {
		return source.Versions[ordinal-1]
	}
	return VersionData{Title: row.Title, Body: source.Body, Tags: row.Tags, Category: row.Category}
}

func revisionForRow(state *PathLedgerState, row PlanRow) *RevisionState {
	if state == nil {
		return nil
	}
	for i := len(state.Revisions) - 1; i >= 0; i-- {
		revision := state.Revisions[i]
		if revision.SourceHash != row.SourceHash {
			continue
		}
		if row.Versions > 0 {
			if revision.ChainLen == row.Versions {
				return revision
			}
			continue
		}
		if revision.ChainLen != 1 {
			continue
		}
		if committed, ok := revision.Committed[1]; ok && committed.ContentHash == row.ContentHash {
			return revision
		}
		if intents := revision.Intents[1]; len(intents) > 0 && intents[len(intents)-1].IntendedContentHash == row.ContentHash {
			return revision
		}
	}
	return nil
}

// liveRevision is the revision a commit has to continue. One the server refused
// before writing anything needs no continuing, so the row starts over.
func liveRevision(state *PathLedgerState, row PlanRow) *RevisionState {
	revision := revisionForRow(state, row)
	if revision != nil && revision.landedNothing() {
		return nil
	}
	return revision
}

func rowRevisionComplete(ledger *Ledger, row PlanRow) bool {
	state := ledger.Paths[row.Path]
	revision := revisionForRow(state, row)
	return revision != nil && revision.CommittedPrefix == revision.ChainLen
}

func nextOrdinal(ledger *Ledger, row PlanRow) int {
	revision := revisionForRow(ledger.Paths[row.Path], row)
	if revision == nil {
		return 1
	}
	return revision.CommittedPrefix + 1
}

func (runner *commitRunner) abort(path string, ordinal int, err error) {
	runner.summary.Aborted = &Abort{Path: path, Ordinal: ordinal, Reason: err.Error()}
}

func (runner *commitRunner) refreshSummary() {
	_ = runner.reloadLedger()
	planSummary := PlanSummary{RowsTotal: len(runner.prepared.plan.Manifest)}
	planSummary.RowsLeftOut = len(leftOutRows(runner.prepared.plan, runner.prepared.ledger))
	rows := RowsSummary{}
	for _, row := range runner.prepared.plan.Manifest {
		complete := rowRevisionComplete(runner.prepared.ledger, row)
		if complete {
			planSummary.RowsCommitted++
			if runner.initialComplete[row.Path] {
				rows.AlreadyCommitted++
			} else {
				rows.CompletedNow++
			}
		} else if row.Action == ActionSkip {
			planSummary.RowsReviewSkip++
			rows.ReviewSkipped++
		} else {
			planSummary.RowsRemaining++
		}
	}
	planSummary.Complete = planSummary.RowsRemaining == 0 && planSummary.RowsLeftOut == 0 &&
		runner.summary.Aborted == nil &&
		len(runner.summary.LinksFailed) == 0
	runner.summary.Rows = rows
	runner.summary.Plan = planSummary
	if runner.summary.LinksFailed == nil {
		runner.summary.LinksFailed = []LinkFailure{}
	}
}
