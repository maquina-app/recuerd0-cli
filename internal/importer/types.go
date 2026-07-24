package importer

import (
	"encoding/json"
	"time"
)

const (
	PlanFormat  = "recuerd0.import.plan"
	PlanVersion = 1

	AdapterObsidianMarkdown = "obsidian_markdown"
	AdapterWorkspaceExport  = "workspace_export"

	ActionCreate  = "create"
	ActionVersion = "version"
	ActionSkip    = "skip"

	ThinHint = "This plan looks thin — refine it by hand or hand it to your agent (see the recuerd0 skill's import protocol)."
)

var validCategories = map[string]bool{
	"decision":   true,
	"discovery":  true,
	"preference": true,
	"general":    true,
}

// Plan is the reviewable, source-backed import manifest.
//
// Keep the declaration order stable: yaml.v3 serializes structs in field order,
// which is part of the deterministic plan format.
type Plan struct {
	Format     string      `yaml:"format" json:"format"`
	Version    int         `yaml:"version" json:"version"`
	SourcePath string      `yaml:"source_path" json:"source_path"`
	Adapter    string      `yaml:"adapter" json:"adapter"`
	Workspace  int64       `yaml:"workspace" json:"workspace"`
	Rules      Rules       `yaml:"rules" json:"rules"`
	Scan       ScanStats   `yaml:"scan" json:"scan"`
	Manifest   []PlanRow   `yaml:"manifest" json:"manifest"`
	Exceptions []Exception `yaml:"exceptions" json:"exceptions"`
}

// Rules are deliberately stored in the plan so review-time changes can be
// applied by a later propose without adding more command flags.
type Rules struct {
	DefaultCategory string            `yaml:"default_category" json:"default_category"`
	Exclude         []string          `yaml:"exclude,omitempty" json:"exclude,omitempty"`
	CategoryMap     map[string]string `yaml:"category_map,omitempty" json:"category_map,omitempty"`
	TagMap          map[string]string `yaml:"tag_map,omitempty" json:"tag_map,omitempty"`
	ignore          []string
}

type ScanStats struct {
	TitlesTotal  int      `yaml:"titles_total" json:"titles_total"`
	TitlesFromH1 int      `yaml:"titles_from_h1" json:"titles_from_h1"`
	Excluded     int      `yaml:"excluded" json:"excluded"`
	Warnings     []string `yaml:"warnings,omitempty" json:"warnings,omitempty"`
}

type PlanRow struct {
	Path              string   `yaml:"path" json:"path"`
	Title             string   `yaml:"title" json:"title"`
	Category          string   `yaml:"category" json:"category"`
	Tags              []string `yaml:"tags" json:"tags"`
	Links             []string `yaml:"links,omitempty" json:"links,omitempty"`
	Action            string   `yaml:"action" json:"action"`
	ContentHash       string   `yaml:"content_hash" json:"content_hash"`
	SourceHash        string   `yaml:"source_hash" json:"source_hash"`
	RowFingerprint    string   `yaml:"row_fingerprint" json:"row_fingerprint"`
	TargetMemoryID    int64    `yaml:"target_memory_id,omitempty" json:"target_memory_id,omitempty"`
	SourceWorkspaceID int64    `yaml:"source_workspace_id,omitempty" json:"source_workspace_id,omitempty"`
	RootID            int64    `yaml:"root_id,omitempty" json:"root_id,omitempty"`
	Versions          int      `yaml:"versions,omitempty" json:"versions,omitempty"`
	Notes             []string `yaml:"notes,omitempty" json:"notes,omitempty"`
}

type Exception struct {
	Path       string  `yaml:"path" json:"path"`
	Kind       string  `yaml:"kind" json:"kind"`
	Resolution string  `yaml:"resolution" json:"resolution"`
	Detail     string  `yaml:"detail,omitempty" json:"detail,omitempty"`
	Candidates []int64 `yaml:"candidates,omitempty" json:"candidates,omitempty"`
}

type Counts struct {
	Create      int `json:"create"`
	Version     int `json:"version"`
	Skip        int `json:"skip"`
	Conflicts   int `json:"conflicts"`
	Unparseable int `json:"unparseable"`
	Excluded    int `json:"excluded"`
}

type Digest struct {
	Adapter         string      `json:"adapter"`
	Counts          Counts      `json:"counts"`
	TitlesFromH1Pct int         `json:"titles_from_h1_pct"`
	LinksProposed   int         `json:"links_proposed"`
	TagsProposed    int         `json:"tags_proposed"`
	Exceptions      []Exception `json:"exceptions"`
	Thin            bool        `json:"thin"`
	Hint            string      `json:"hint,omitempty"`
	Warnings        []string    `json:"warnings"`
}

type ProposeOptions struct {
	SourcePath string
	PlanPath   string
	LedgerPath string
	Adapter    string
	Workspace  int64
	Fresh      bool
}

type CommitOptions struct {
	PlanPath   string
	LedgerPath string
}

type OpsSummary struct {
	Created    int `json:"created"`
	Versioned  int `json:"versioned"`
	Reconciled int `json:"reconciled"`
}

type RowsSummary struct {
	CompletedNow     int `json:"completed_now"`
	ReviewSkipped    int `json:"review_skipped"`
	AlreadyCommitted int `json:"already_committed"`
}

type PlanSummary struct {
	RowsTotal      int  `json:"rows_total"`
	RowsCommitted  int  `json:"rows_committed"`
	RowsReviewSkip int  `json:"rows_review_skip"`
	RowsRemaining  int  `json:"rows_remaining"`
	Complete       bool `json:"complete"`
}

type LinksEnsured struct {
	Created  int `json:"created"`
	Existing int `json:"existing"`
}

type LinkFailure struct {
	FromPath string `json:"from_path"`
	ToPath   string `json:"to_path"`
	Reason   string `json:"reason"`
}

type Abort struct {
	Path    string `json:"path,omitempty"`
	Ordinal int    `json:"ordinal,omitempty"`
	Reason  string `json:"reason"`
}

type CommitSummary struct {
	Ops                      OpsSummary    `json:"ops"`
	Rows                     RowsSummary   `json:"rows"`
	Plan                     PlanSummary   `json:"plan"`
	LinksEnsured             LinksEnsured  `json:"links_ensured"`
	LinksSkippedUnresolvable int           `json:"links_skipped_unresolvable"`
	LinksFailed              []LinkFailure `json:"links_failed"`
	LedgerPath               string        `json:"ledger_path"`
	Aborted                  *Abort        `json:"aborted,omitempty"`
}

// LedgerRecord is the append-only wire format for both intent and committed
// records. Fields that do not apply to a kind are omitted.
type LedgerRecord struct {
	Kind                string    `json:"kind"`
	Path                string    `json:"path"`
	MemoryID            int64     `json:"memory_id,omitempty"`
	Workspace           int64     `json:"workspace"`
	Action              string    `json:"action"`
	Ordinal             int       `json:"ordinal"`
	ChainLen            int       `json:"chain_len"`
	SourceHash          string    `json:"source_hash"`
	IntendedContentHash string    `json:"intended_content_hash,omitempty"`
	ContentHash         string    `json:"content_hash,omitempty"`
	ExpectedVersion     int       `json:"expected_version,omitempty"`
	Version             int       `json:"version,omitempty"`
	ChainBase           int       `json:"chain_base"`
	Reconciled          bool      `json:"reconciled,omitempty"`
	AttemptedAt         time.Time `json:"attempted_at,omitempty"`
	CommittedAt         time.Time `json:"committed_at,omitempty"`
	Line                int       `json:"-"`
}

// MarshalJSON uses pointer timestamps on the wire so the timestamp belonging
// to the other record kind is actually omitted (time.Time is not empty under
// encoding/json's ordinary omitempty rules).
func (record LedgerRecord) MarshalJSON() ([]byte, error) {
	type ledgerWire struct {
		Kind                string     `json:"kind"`
		Path                string     `json:"path"`
		MemoryID            int64      `json:"memory_id,omitempty"`
		Workspace           int64      `json:"workspace"`
		Action              string     `json:"action"`
		Ordinal             int        `json:"ordinal"`
		ChainLen            int        `json:"chain_len"`
		SourceHash          string     `json:"source_hash"`
		IntendedContentHash string     `json:"intended_content_hash,omitempty"`
		ContentHash         string     `json:"content_hash,omitempty"`
		ExpectedVersion     int        `json:"expected_version,omitempty"`
		Version             int        `json:"version,omitempty"`
		ChainBase           int        `json:"chain_base"`
		Reconciled          bool       `json:"reconciled,omitempty"`
		AttemptedAt         *time.Time `json:"attempted_at,omitempty"`
		CommittedAt         *time.Time `json:"committed_at,omitempty"`
	}
	wire := ledgerWire{
		Kind: record.Kind, Path: record.Path, MemoryID: record.MemoryID,
		Workspace: record.Workspace, Action: record.Action, Ordinal: record.Ordinal,
		ChainLen: record.ChainLen, SourceHash: record.SourceHash,
		IntendedContentHash: record.IntendedContentHash, ContentHash: record.ContentHash,
		ExpectedVersion: record.ExpectedVersion, Version: record.Version,
		ChainBase: record.ChainBase, Reconciled: record.Reconciled,
	}
	if !record.AttemptedAt.IsZero() {
		wire.AttemptedAt = &record.AttemptedAt
	}
	if !record.CommittedAt.IsZero() {
		wire.CommittedAt = &record.CommittedAt
	}
	return json.Marshal(wire)
}

// VersionData is a normalized source version used by both adapters.
type VersionData struct {
	Title    string
	Body     string
	Tags     []string
	Category string
}

type sourceRow struct {
	Path              string
	Title             string
	Body              string
	Tags              []string
	Category          string
	Links             []string
	SourceHash        string
	FromH1            bool
	SourceWorkspaceID int64
	RootID            int64
	Versions          []VersionData
	Notes             []string
	Exceptions        []Exception
}

type scanResult struct {
	Rows    []sourceRow
	Stats   ScanStats
	Adapter string
}
