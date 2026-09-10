package commands

import (
	"fmt"
	"strings"
)

// ValidIncludeTokens is the locked enum of `--include` tokens supported by the
// recuerd0 server. Keep in sync with ObsoleteFilterable on the Rails side.
//
// Retrieval hides memories tagged obsolete/superseded/deprecated by default;
// `--include obsolete` brings them back for an audit.
var ValidIncludeTokens = []string{"obsolete"}

// NormalizeInclude splits a comma-separated --include value into trimmed,
// lowercased tokens, dropping empties. The server parses the param the same way.
func NormalizeInclude(s string) []string {
	tokens := []string{}
	for _, raw := range strings.Split(s, ",") {
		token := strings.ToLower(strings.TrimSpace(raw))
		if token != "" {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

// IsValidInclude reports whether every token in s is supported. Empty string
// returns true (means "server default": obsolete memories stay hidden).
//
// An unrecognized token is not an error: the caller asked to widen retrieval by
// a name that matches nothing, so the honest answer is an empty result set, not
// the filtered list they were trying to widen. See UnknownIncludeTokens.
func IsValidInclude(s string) bool {
	for _, token := range NormalizeInclude(s) {
		if !containsToken(ValidIncludeTokens, token) {
			return false
		}
	}
	return true
}

func containsToken(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// includeParam returns the canonical value to send as the `include` query
// param, or "" when nothing should be sent.
func includeParam(s string) string {
	return strings.Join(NormalizeInclude(s), ",")
}

// UnknownIncludeTokens returns the tokens in s the CLI does not recognize, in
// the order given.
func UnknownIncludeTokens(s string) []string {
	unknown := []string{}
	for _, token := range NormalizeInclude(s) {
		if !containsToken(ValidIncludeTokens, token) {
			unknown = append(unknown, token)
		}
	}
	return unknown
}

// unknownIncludeSummary explains an empty result set caused by an --include
// value that matches nothing, naming the offending tokens so a typo is visible
// rather than mistaken for "the workspace has none of those."
func unknownIncludeSummary(s string) string {
	return fmt.Sprintf(
		"0 result(s): --include %s matches nothing (supported: %s)",
		strings.Join(UnknownIncludeTokens(s), ", "),
		strings.Join(ValidIncludeTokens, ", "),
	)
}

// countObsoleteHidden reads `obsolete_hidden` off a search payload — how many
// retired memories the server withheld from these results. Absent or
// unparseable means zero.
func countObsoleteHidden(data interface{}) int {
	m, ok := data.(map[string]interface{})
	if !ok {
		return 0
	}
	switch v := m["obsolete_hidden"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	default:
		return 0
	}
}
