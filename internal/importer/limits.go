package importer

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// What the server refuses or rewrites on a write. Propose has to know this:
// commit stops at the first refusal, so anything left for the server to catch
// strands every row after it.
const (
	maxTitleLength  = 255
	maxTags         = 50
	maxTagLength    = 100
	maxContentBytes = 1 << 20
)

var (
	listItemLine = regexp.MustCompile(`^[\t\n\v\f\r ]*(?:[-*+]|[0-9]+[.)])[\t\n\v\f\r ]`)
	fenceLine    = regexp.MustCompile("^(?:```|~~~)")
)

// distinctTags is what the server stores: stripped, no blanks, and tags that
// differ only in case count as one, with the first spelling kept.
func distinctTags(tags []string) []string {
	seen := make(map[string]bool, len(tags))
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		key := strings.ToLower(tag)
		if tag == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, tag)
	}
	return result
}

// storedBody is what the server stores for a body: a fence straight after a list
// gets a blank line before it. It mirrors ActionText::Markdown.separate_fences_from_lists
// and, like that method, is idempotent, so it can compare text from servers with
// or without the rule.
func storedBody(body string) string {
	var output strings.Builder
	output.Grow(len(body))
	inFence, inList := false, false
	for _, line := range strings.SplitAfter(body, "\n") {
		switch {
		case inFence:
			if fenceLine.MatchString(line) {
				inFence = false
			}
		case fenceLine.MatchString(line):
			if inList {
				output.WriteByte('\n')
			}
			inFence, inList = true, false
		case strings.TrimSpace(line) == "":
			inList = false
		case listItemLine.MatchString(line):
			inList = true
		}
		output.WriteString(line)
	}
	return output.String()
}

func blank(text string) bool {
	return strings.TrimSpace(text) == ""
}

// sameVersion is the server's test for a version that changes nothing.
func sameVersion(first, second VersionData) bool {
	return first.Title == second.Title && first.Category == second.Category &&
		equalStringSlices(distinctTags(first.Tags), distinctTags(second.Tags)) &&
		storedBody(first.Body) == storedBody(second.Body)
}

// contentProblems lists why the server would refuse these tags and body.
func contentProblems(tags []string, body string) []string {
	var problems []string
	distinct := distinctTags(tags)
	if len(distinct) > maxTags {
		problems = append(problems, fmt.Sprintf("%d tags (maximum %d)", len(distinct), maxTags))
	}
	for _, tag := range distinct {
		if length := utf8.RuneCountInString(tag); length > maxTagLength {
			problems = append(problems, fmt.Sprintf("a tag of %d characters (maximum %d)", length, maxTagLength))
			break
		}
	}
	if len(body) > maxContentBytes {
		problems = append(problems, fmt.Sprintf("%d bytes of content (maximum %d)", len(body), maxContentBytes))
	}
	if strings.ContainsRune(body, 0) {
		problems = append(problems, "a null character in its content")
	}
	if !utf8.ValidString(body) {
		problems = append(problems, "content that isn't valid UTF-8")
	}
	return problems
}

// applyLimitExceptions skips rows the server would refuse. It runs after merging,
// so tags a reviewer trimmed in the plan count rather than the source's. A row the
// ledger has written is left alone: it is either finished or has to resume.
func applyLimitExceptions(plan *Plan, scanned []sourceRow, ledger *Ledger) {
	sources := make(map[string]sourceRow, len(scanned))
	for _, source := range scanned {
		sources[source.Path] = source
	}
	for _, row := range plan.Manifest {
		if liveRevision(ledger.Paths[row.Path], row) != nil {
			continue
		}
		source := sources[row.Path]
		if plan.Adapter == AdapterWorkspaceExport {
			plan.Exceptions = append(plan.Exceptions, exportLimitExceptions(source)...)
		} else {
			problems := contentProblems(row.Tags, source.Body)
			if row.Action == ActionVersion && blank(source.Body) {
				problems = append(problems, "a blank version over existing content")
			}
			plan.Exceptions = append(plan.Exceptions, unimportable(row.Path, problems)...)
		}
	}
}

func unimportable(path string, problems []string) []Exception {
	if len(problems) == 0 {
		return nil
	}
	return []Exception{{
		Path: path, Kind: "unimportable", Resolution: ActionSkip,
		Detail: "the API refuses this: " + strings.Join(problems, "; "),
	}}
}

// exportLimitExceptions checks every version, because a chain can't lose a middle
// version to a limit without misrepresenting it. Any refusal skips the whole memory.
func exportLimitExceptions(source sourceRow) []Exception {
	for _, exception := range source.Exceptions {
		if exception.Kind == "unimportable" {
			return nil
		}
	}
	var problems []string
	longestTitle := 0
	for i, version := range source.Versions {
		for _, problem := range contentProblems(version.Tags, version.Body) {
			problems = append(problems, fmt.Sprintf("version %d has %s", i+1, problem))
		}
		if length := utf8.RuneCountInString(version.Title); length > longestTitle {
			longestTitle = length
		}
	}
	exceptions := unimportable(source.Path, problems)
	if longestTitle > maxTitleLength {
		exceptions = append(exceptions, Exception{
			Path: source.Path, Kind: "title_too_long", Resolution: ActionSkip,
			Detail: fmt.Sprintf("title is %d characters (maximum %d)", longestTitle, maxTitleLength),
		})
	}
	return exceptions
}

// chainForImport drops the versions an older server allowed and this one refuses:
// repeats of the version before, and blanks that would overwrite content. Neither
// holds text worth keeping, but numbers shift, so both are reported for review.
func chainForImport(path string, versions []VersionData) ([]VersionData, []Exception) {
	kept := make([]VersionData, 0, len(versions))
	var repeated, blanks []string
	for i, version := range versions {
		number := fmt.Sprintf("%d", i+1)
		if len(kept) == 0 {
			if blank(version.Title) && blank(version.Body) {
				blanks = append(blanks, number)
			} else {
				kept = append(kept, version)
			}
			continue
		}
		last := kept[len(kept)-1]
		switch {
		case blank(version.Body) && !blank(last.Body):
			blanks = append(blanks, number)
		case sameVersion(last, version):
			repeated = append(repeated, number)
		default:
			kept = append(kept, version)
		}
	}
	if len(kept) == 0 {
		return versions, []Exception{{
			Path: path, Kind: "unimportable", Resolution: ActionSkip,
			Detail: "the API refuses this: every version has a blank title and blank content",
		}}
	}
	var exceptions []Exception
	if len(blanks) > 0 {
		detail := fmt.Sprintf("version(s) %s are blank and won't be imported, so %d of %d version(s) land and numbers shift",
			strings.Join(blanks, ", "), len(kept), len(versions))
		if blank(versions[len(versions)-1].Body) {
			detail += "; the source's current version is blank, and the imported memory ends on the last one with content"
		}
		exceptions = append(exceptions, Exception{Path: path, Kind: "blank_versions", Resolution: ActionCreate, Detail: detail})
	}
	if len(repeated) > 0 {
		exceptions = append(exceptions, Exception{
			Path: path, Kind: "repeated_versions", Resolution: ActionCreate,
			Detail: fmt.Sprintf("version(s) %s repeat the version before and won't be imported, so %d of %d version(s) land and numbers shift",
				strings.Join(repeated, ", "), len(kept), len(versions)),
		})
	}
	return kept, exceptions
}
