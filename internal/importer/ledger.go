package importer

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

type Ledger struct {
	Records []LedgerRecord
	Paths   map[string]*PathLedgerState
}

type PathLedgerState struct {
	Path        string
	MemoryID    int64
	Revisions   []*RevisionState
	RecordLines []int
}

type RevisionState struct {
	FirstLine       int
	SourceHash      string
	Action          string
	ChainLen        int
	ChainBase       int
	MemoryID        int64
	Committed       map[int]LedgerRecord
	Intents         map[int][]LedgerRecord
	CommittedPrefix int
	LastCommitted   *LedgerRecord
	UnmatchedIntent *LedgerRecord
}

// LoadLedger treats a missing path as a first run. Existing empty ledgers are
// valid; unreadable and malformed files are hard errors.
func LoadLedger(path string) (*Ledger, error) {
	ledger := &Ledger{Paths: make(map[string]*PathLedgerState)}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return ledger, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	line := 0
	for scanner.Scan() {
		line++
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			return nil, validationf("ledger line %d: empty record", line)
		}
		var record LedgerRecord
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&record); err != nil {
			return nil, validationf("ledger line %d: %v", line, err)
		}
		var trailing interface{}
		if err := decoder.Decode(&trailing); err == nil {
			return nil, validationf("ledger line %d: multiple JSON values", line)
		} else if err != io.EOF {
			return nil, validationf("ledger line %d: %v", line, err)
		}
		record.Line = line
		if err := validateLedgerRecord(record); err != nil {
			return nil, validationf("ledger line %d: %v", line, err)
		}
		ledger.Records = append(ledger.Records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}
	if err := ledger.reconstruct(); err != nil {
		return nil, err
	}
	return ledger, nil
}

func validateLedgerRecord(record LedgerRecord) error {
	if record.Kind != "intent" && record.Kind != "committed" {
		return fmt.Errorf("kind must be intent or committed")
	}
	if record.Path == "" || record.Workspace <= 0 || !validAction(record.Action) || record.Action == ActionSkip {
		return fmt.Errorf("invalid path, workspace, or action")
	}
	if record.Ordinal <= 0 || record.ChainLen <= 0 || record.Ordinal > record.ChainLen {
		return fmt.Errorf("invalid ordinal %d/%d", record.Ordinal, record.ChainLen)
	}
	if !hashPattern.MatchString(record.SourceHash) {
		return fmt.Errorf("invalid source_hash")
	}
	if record.ChainBase < 0 {
		return fmt.Errorf("chain_base cannot be negative")
	}
	if record.Kind == "intent" {
		if !hashPattern.MatchString(record.IntendedContentHash) || record.ExpectedVersion <= 0 || record.AttemptedAt.IsZero() {
			return fmt.Errorf("intent requires intended_content_hash, expected_version, and attempted_at")
		}
		if record.ContentHash != "" || record.Version != 0 || record.Reconciled || !record.CommittedAt.IsZero() {
			return fmt.Errorf("intent contains committed-only fields")
		}
	} else {
		if record.MemoryID <= 0 || !hashPattern.MatchString(record.ContentHash) || record.Version <= 0 || record.CommittedAt.IsZero() {
			return fmt.Errorf("committed requires memory_id, content_hash, version, and committed_at")
		}
		if record.IntendedContentHash != "" || record.ExpectedVersion != 0 || !record.AttemptedAt.IsZero() {
			return fmt.Errorf("committed contains intent-only fields")
		}
	}
	return nil
}

func (ledger *Ledger) reconstruct() error {
	ledger.Paths = make(map[string]*PathLedgerState)
	var problems []string
	for _, record := range ledger.Records {
		pathState := ledger.Paths[record.Path]
		if pathState == nil {
			pathState = &PathLedgerState{Path: record.Path}
			ledger.Paths[record.Path] = pathState
		}
		pathState.RecordLines = append(pathState.RecordLines, record.Line)
		if pathState.MemoryID != 0 && record.MemoryID != 0 && pathState.MemoryID != record.MemoryID {
			appendProblem(&problems, record.Line, "memory_id %d for %q is immutable (previously %d)", record.MemoryID, record.Path, pathState.MemoryID)
		}
		if record.MemoryID != 0 {
			pathState.MemoryID = record.MemoryID
		}
		var revision *RevisionState
		if len(pathState.Revisions) > 0 {
			revision = pathState.Revisions[len(pathState.Revisions)-1]
		}
		if revision == nil || (record.Kind == "intent" && record.Ordinal == 1 && revisionRecordsComplete(revision)) {
			revision = &RevisionState{
				FirstLine:  record.Line,
				SourceHash: record.SourceHash,
				Action:     record.Action,
				ChainLen:   record.ChainLen,
				ChainBase:  record.ChainBase,
				Committed:  make(map[int]LedgerRecord),
				Intents:    make(map[int][]LedgerRecord),
			}
			pathState.Revisions = append(pathState.Revisions, revision)
		} else {
			if revision.SourceHash != record.SourceHash {
				appendProblem(&problems, record.Line, "source_hash for active chain %q is immutable", record.Path)
			}
			if revision.Action != record.Action {
				appendProblem(&problems, record.Line, "action for %q source revision is immutable (%q, got %q)", record.Path, revision.Action, record.Action)
			}
			if revision.ChainLen != record.ChainLen {
				appendProblem(&problems, record.Line, "chain_len for %q source revision is immutable (%d, got %d)", record.Path, revision.ChainLen, record.ChainLen)
			}
			if revision.ChainBase != record.ChainBase {
				appendProblem(&problems, record.Line, "chain_base for %q source revision is immutable (%d, got %d)", record.Path, revision.ChainBase, record.ChainBase)
			}
		}
		if revision.MemoryID != 0 && record.MemoryID != 0 && revision.MemoryID != record.MemoryID {
			appendProblem(&problems, record.Line, "memory_id for %q source revision is immutable", record.Path)
		}
		if record.MemoryID != 0 {
			revision.MemoryID = record.MemoryID
		}
		if record.Kind == "intent" {
			existing := revision.Intents[record.Ordinal]
			if len(existing) > 0 {
				first := existing[0]
				if first.ExpectedVersion != record.ExpectedVersion ||
					first.IntendedContentHash != record.IntendedContentHash ||
					(first.MemoryID != 0 && record.MemoryID != 0 && first.MemoryID != record.MemoryID) {
					appendProblem(&problems, record.Line, "intent for %q ordinal %d disagrees with prior intent", record.Path, record.Ordinal)
				}
			}
			revision.Intents[record.Ordinal] = append(existing, record)
		} else {
			if _, exists := revision.Committed[record.Ordinal]; exists {
				appendProblem(&problems, record.Line, "duplicate committed record for %q ordinal %d", record.Path, record.Ordinal)
			} else {
				revision.Committed[record.Ordinal] = record
			}
		}
	}
	for _, pathState := range ledger.Paths {
		for _, revision := range pathState.Revisions {
			for ordinal := 1; ordinal <= revision.ChainLen; ordinal++ {
				committed, ok := revision.Committed[ordinal]
				if !ok {
					for later := ordinal + 1; later <= revision.ChainLen; later++ {
						if _, exists := revision.Committed[later]; exists {
							appendProblem(&problems, revision.Committed[later].Line, "committed chain for %q has a gap before ordinal %d", pathState.Path, later)
						}
					}
					break
				}
				intents := revision.Intents[ordinal]
				if len(intents) == 0 {
					appendProblem(&problems, committed.Line, "committed record for %q ordinal %d has no preceding intent", pathState.Path, ordinal)
				} else {
					preceding := false
					for _, intent := range intents {
						if intent.Line < committed.Line {
							preceding = true
							if intent.IntendedContentHash != committed.ContentHash {
								appendProblem(&problems, committed.Line, "committed content_hash for %q ordinal %d disagrees with intent", pathState.Path, ordinal)
							}
						}
					}
					if !preceding {
						appendProblem(&problems, committed.Line, "committed record for %q ordinal %d precedes its intent", pathState.Path, ordinal)
					}
				}
				expected := revision.ChainBase + ordinal
				if committed.Version != expected {
					appendProblem(&problems, committed.Line, "version arithmetic for %q ordinal %d: expected %d, got %d", pathState.Path, ordinal, expected, committed.Version)
				}
				revision.CommittedPrefix = ordinal
				copy := committed
				revision.LastCommitted = &copy
			}
			next := revision.CommittedPrefix + 1
			if next <= revision.ChainLen {
				if intents := revision.Intents[next]; len(intents) > 0 {
					last := intents[len(intents)-1]
					copy := last
					revision.UnmatchedIntent = &copy
				}
			}
			for ordinal, intents := range revision.Intents {
				if ordinal > revision.CommittedPrefix+1 {
					appendProblem(&problems, intents[0].Line, "intent for %q ordinal %d skips the next ordinal", pathState.Path, ordinal)
				}
				expected := revision.ChainBase + ordinal
				for _, intent := range intents {
					if intent.ExpectedVersion != expected {
						appendProblem(&problems, intent.Line, "expected_version for %q ordinal %d must be %d, got %d", pathState.Path, ordinal, expected, intent.ExpectedVersion)
					}
				}
			}
		}
	}
	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

// AppendLedgerRecord opens, appends, fsyncs, and closes for every record.
func AppendLedgerRecord(path string, record LedgerRecord) error {
	if err := validateLedgerRecord(record); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create ledger directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open ledger: %w", err)
	}
	data, err := json.Marshal(record)
	if err == nil {
		data = append(data, '\n')
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return fmt.Errorf("append ledger: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("close ledger: %w", closeErr)
	}
	return nil
}

func newIntent(path string, workspace int64, action string, ordinal, chainLen int, sourceHash, contentHash string, expectedVersion, chainBase int, memoryID int64) LedgerRecord {
	return LedgerRecord{
		Kind: "intent", Path: path, MemoryID: memoryID, Workspace: workspace,
		Action: action, Ordinal: ordinal, ChainLen: chainLen, SourceHash: sourceHash,
		IntendedContentHash: contentHash, ExpectedVersion: expectedVersion,
		ChainBase: chainBase, AttemptedAt: time.Now().UTC(),
	}
}

func newCommitted(intent LedgerRecord, memoryID int64, reconciled bool) LedgerRecord {
	return LedgerRecord{
		Kind: "committed", Path: intent.Path, MemoryID: memoryID,
		Workspace: intent.Workspace, Action: intent.Action, Ordinal: intent.Ordinal,
		ChainLen: intent.ChainLen, SourceHash: intent.SourceHash,
		ContentHash: intent.IntendedContentHash, Version: intent.ExpectedVersion,
		ChainBase: intent.ChainBase, Reconciled: reconciled, CommittedAt: time.Now().UTC(),
	}
}

func (ledger *Ledger) identityMap() map[string]int64 {
	result := make(map[string]int64)
	for path, state := range ledger.Paths {
		result[path] = state.MemoryID
	}
	return result
}

func sortedRevisionStates(state *PathLedgerState) []*RevisionState {
	if state == nil {
		return nil
	}
	return append([]*RevisionState(nil), state.Revisions...)
}

func revisionRecordsComplete(revision *RevisionState) bool {
	if revision == nil || len(revision.Committed) != revision.ChainLen {
		return false
	}
	for ordinal := 1; ordinal <= revision.ChainLen; ordinal++ {
		if _, ok := revision.Committed[ordinal]; !ok {
			return false
		}
	}
	return true
}
