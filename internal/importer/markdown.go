package importer

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const invalidCategoryNote = "invalid frontmatter category — using path/default category"

var (
	wikiLinkPattern     = regexp.MustCompile(`(!?)\[\[([^\]]+)\]\]`)
	markdownLinkPattern = regexp.MustCompile(`(!?)\[[^\]]*\]\(([^)]+)\)`)
)

type rawMarkdownRow struct {
	sourceRow
	rawLinks []rawLink
}

type rawLink struct {
	target string
	embed  bool
	wiki   bool
}

func defaultRules() Rules {
	return Rules{
		DefaultCategory: "general",
		Exclude: []string{
			"**/.DS_Store",
			"**/Thumbs.db",
			"**/node_modules/**",
		},
	}
}

func scanMarkdown(root string, rules Rules) (*scanResult, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("inspect markdown source: %w", err)
	}
	if !info.IsDir() {
		return nil, validationf("obsidian_markdown source must be a directory")
	}
	if !validCategories[rules.DefaultCategory] {
		return nil, validationf("invalid default category %q", rules.DefaultCategory)
	}

	var files []string
	excluded := 0
	var excludedPaths []string
	err = filepath.WalkDir(root, func(fullPath string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if fullPath == root {
			return nil
		}
		relative, err := filepath.Rel(root, fullPath)
		if err != nil {
			return err
		}
		manifestPath := filepath.ToSlash(relative)
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || matchesAnyGlob(manifestPath+"/", rules.Exclude) {
				return fs.SkipDir
			}
			return nil
		}
		if matchesAnyGlob(manifestPath, rules.ignore) {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") || matchesAnyGlob(manifestPath, rules.Exclude) {
			excluded++
			excludedPaths = append(excludedPaths, manifestPath)
			return nil
		}
		if strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			files = append(files, manifestPath)
		} else {
			excluded++
			excludedPaths = append(excludedPaths, manifestPath)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk markdown source: %w", err)
	}
	sort.Strings(files)

	rawRows := make([]rawMarkdownRow, 0, len(files))
	for _, manifestPath := range files {
		fullPath := filepath.Join(root, filepath.FromSlash(manifestPath))
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", manifestPath, err)
		}
		row := parseMarkdownFile(manifestPath, string(data), rules)
		rawRows = append(rawRows, row)
	}

	resolveMarkdownLinks(rawRows)
	rows := make([]sourceRow, len(rawRows))
	for i := range rawRows {
		rows[i] = rawRows[i].sourceRow
	}
	applyDuplicateExceptions(rows)
	sort.Strings(excludedPaths)
	warnings := make([]string, 0, len(excludedPaths))
	for _, excludedPath := range excludedPaths {
		warnings = append(warnings, "excluded file: "+excludedPath)
	}
	stats := ScanStats{TitlesTotal: len(rows), Excluded: excluded, Warnings: warnings}
	for _, row := range rows {
		if row.FromH1 {
			stats.TitlesFromH1++
		}
	}
	return &scanResult{Rows: rows, Stats: stats, Adapter: AdapterObsidianMarkdown}, nil
}

func parseMarkdownFile(manifestPath, content string, rules Rules) rawMarkdownRow {
	frontmatter, body, frontmatterErr := splitFrontmatter(content)
	var metadata struct {
		Tags     interface{} `yaml:"tags"`
		Category string      `yaml:"category"`
	}
	var exceptions []Exception
	if frontmatterErr == nil && frontmatter != "" {
		if err := yaml.Unmarshal([]byte(frontmatter), &metadata); err != nil {
			frontmatterErr = err
		}
	}
	if frontmatterErr != nil {
		exceptions = append(exceptions, Exception{
			Path: manifestPath, Kind: "unparseable", Resolution: ActionSkip,
			Detail: frontmatterErr.Error(),
		})
	}

	title, fromH1 := firstH1(body)
	if title == "" {
		title = strings.TrimSuffix(path.Base(manifestPath), path.Ext(manifestPath))
		exceptions = append(exceptions, Exception{
			Path: manifestPath, Kind: "guessed_title", Resolution: ActionCreate,
			Detail: "no non-fenced H1 heading",
		})
	}
	if len([]rune(title)) > 255 {
		exceptions = append(exceptions, Exception{
			Path: manifestPath, Kind: "title_too_long", Resolution: ActionSkip,
			Detail: fmt.Sprintf("title is %d characters (maximum 255)", len([]rune(title))),
		})
	}

	var contributions []string
	directory := path.Dir(manifestPath)
	if directory != "." {
		contributions = append(contributions, strings.Split(directory, "/")...)
	}
	if frontmatterErr == nil {
		contributions = append(contributions, frontmatterTags(metadata.Tags)...)
	}
	tags := normalizeTags(contributions, rules.TagMap)
	category, notes := chooseCategory(manifestPath, metadata.Category, frontmatterErr == nil, rules)
	rawLinks, linkNotes := extractRawLinks(body)
	notes = append(notes, linkNotes...)

	return rawMarkdownRow{
		sourceRow: sourceRow{
			Path: manifestPath, Title: title, Body: body, Tags: tags,
			Category: category, SourceHash: sourceBodyHash(body), FromH1: fromH1,
			Notes: notes, Exceptions: exceptions,
		},
		rawLinks: rawLinks,
	}
}

func splitFrontmatter(content string) (frontmatter, body string, err error) {
	if !(strings.HasPrefix(content, "---\n") || strings.HasPrefix(content, "---\r\n")) {
		return "", content, nil
	}
	offset := strings.IndexByte(content, '\n') + 1
	position := offset
	for position <= len(content) {
		next := strings.IndexByte(content[position:], '\n')
		var line string
		var end int
		if next < 0 {
			line = content[position:]
			end = len(content)
		} else {
			line = content[position : position+next]
			end = position + next + 1
		}
		if strings.TrimSuffix(line, "\r") == "---" {
			return content[offset:position], content[end:], nil
		}
		if next < 0 {
			break
		}
		position = end
	}
	return "", content, fmt.Errorf("unterminated YAML frontmatter")
}

func firstH1(body string) (string, bool) {
	inFence := false
	fenceChar := byte(0)
	fenceLen := 0
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimLeft(line, " \t")
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			char := trimmed[0]
			count := 0
			for count < len(trimmed) && trimmed[count] == char {
				count++
			}
			if !inFence {
				inFence, fenceChar, fenceLen = true, char, count
			} else if char == fenceChar && count >= fenceLen {
				inFence = false
			}
			continue
		}
		if !inFence && strings.HasPrefix(line, "# ") {
			if title := strings.TrimSpace(strings.TrimPrefix(line, "# ")); title != "" {
				return title, true
			}
		}
	}
	return "", false
}

func frontmatterTags(value interface{}) []string {
	switch typed := value.(type) {
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return result
	case []string:
		return append([]string(nil), typed...)
	case string:
		return strings.FieldsFunc(typed, func(r rune) bool {
			return r == ',' || unicode.IsSpace(r)
		})
	default:
		return nil
	}
}

func normalizeTags(contributions []string, tagMap map[string][]string) []string {
	seen := make(map[string]bool)
	for _, contribution := range contributions {
		mappedValues, exists := tagMap[contribution]
		if !exists {
			mappedValues = []string{contribution}
		}
		for _, mapped := range mappedValues {
			mapped = strings.TrimSpace(mapped)
			if mapped == "" {
				continue
			}
			var builder strings.Builder
			underscore := false
			for _, r := range strings.ToLower(mapped) {
				if unicode.IsSpace(r) || r == '-' {
					if builder.Len() > 0 {
						underscore = true
					}
					continue
				}
				if underscore {
					builder.WriteByte('_')
					underscore = false
				}
				builder.WriteRune(r)
			}
			tag := strings.Trim(builder.String(), "_")
			if tag != "" {
				seen[tag] = true
			}
		}
	}
	result := make([]string, 0, len(seen))
	for tag := range seen {
		result = append(result, tag)
	}
	sort.Strings(result)
	return result
}

func chooseCategory(manifestPath, frontmatter string, frontmatterValid bool, rules Rules) (string, []string) {
	if frontmatterValid && validCategories[frontmatter] {
		return frontmatter, nil
	}
	var notes []string
	if frontmatterValid && frontmatter != "" && !validCategories[frontmatter] {
		notes = append(notes, invalidCategoryNote)
	}
	bestPrefix := ""
	bestCategory := ""
	for prefix, category := range rules.CategoryMap {
		normalized := strings.Trim(strings.ReplaceAll(prefix, `\`, "/"), "/")
		if manifestPath == normalized || strings.HasPrefix(manifestPath, normalized+"/") {
			if len(normalized) > len(bestPrefix) {
				bestPrefix, bestCategory = normalized, category
			}
		}
	}
	if bestCategory != "" {
		return bestCategory, notes
	}
	return rules.DefaultCategory, notes
}

func extractRawLinks(body string) ([]rawLink, []string) {
	var links []rawLink
	var notes []string
	for _, match := range wikiLinkPattern.FindAllStringSubmatch(body, -1) {
		target := strings.TrimSpace(strings.SplitN(strings.SplitN(match[2], "|", 2)[0], "#", 2)[0])
		if target == "" {
			continue
		}
		embed := match[1] == "!"
		links = append(links, rawLink{target: target, embed: embed, wiki: true})
		if embed {
			notes = append(notes, "embed excluded: "+target)
		}
	}
	for _, match := range markdownLinkPattern.FindAllStringSubmatch(body, -1) {
		target := strings.TrimSpace(strings.Trim(match[2], "<>"))
		if blank := strings.IndexAny(target, " \t"); blank >= 0 {
			target = target[:blank]
		}
		target = strings.SplitN(target, "#", 2)[0]
		if decoded, err := url.PathUnescape(target); err == nil {
			target = decoded
		}
		if target == "" || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		embed := match[1] == "!"
		links = append(links, rawLink{target: target, embed: embed})
		if embed {
			notes = append(notes, "image reference excluded: "+target)
		}
	}
	sort.Strings(notes)
	return links, uniqueStrings(notes)
}

func resolveMarkdownLinks(rows []rawMarkdownRow) {
	full := make(map[string][]string)
	base := make(map[string][]string)
	for _, row := range rows {
		lower := strings.ToLower(row.Path)
		full[lower] = append(full[lower], row.Path)
		baseName := strings.ToLower(strings.TrimSuffix(path.Base(row.Path), path.Ext(row.Path)))
		base[baseName] = append(base[baseName], row.Path)
	}
	for i := range rows {
		var resolved []string
		var notes []string
		for _, link := range rows[i].rawLinks {
			if link.embed {
				continue
			}
			target := strings.ReplaceAll(link.target, `\`, "/")
			var candidates []string
			if link.wiki {
				if strings.Contains(target, "/") {
					if !strings.HasSuffix(strings.ToLower(target), ".md") {
						target += ".md"
					}
					target = strings.TrimPrefix(path.Clean(target), "./")
					candidates = full[strings.ToLower(target)]
				} else {
					name := strings.TrimSuffix(path.Base(target), path.Ext(target))
					candidates = base[strings.ToLower(name)]
				}
			} else if strings.Contains(target, "/") || strings.HasPrefix(target, ".") || strings.HasSuffix(strings.ToLower(target), ".md") {
				if !strings.HasSuffix(strings.ToLower(target), ".md") {
					target += ".md"
				}
				joined := path.Clean(path.Join(path.Dir(rows[i].Path), target))
				candidates = full[strings.ToLower(strings.TrimPrefix(joined, "./"))]
			} else {
				name := strings.TrimSuffix(path.Base(target), path.Ext(target))
				candidates = base[strings.ToLower(name)]
			}
			switch len(candidates) {
			case 0:
				notes = append(notes, "unresolved link: "+link.target)
			case 1:
				if candidates[0] != rows[i].Path {
					resolved = append(resolved, candidates[0])
				}
			default:
				sorted := append([]string(nil), candidates...)
				sort.Strings(sorted)
				notes = append(notes, "ambiguous link: "+link.target+" -> "+strings.Join(sorted, ", "))
			}
		}
		sort.Strings(resolved)
		rows[i].Links = uniqueStrings(resolved)
		sort.Strings(notes)
		rows[i].Notes = append(rows[i].Notes, uniqueStrings(notes)...)
		rows[i].Notes = uniqueStrings(rows[i].Notes)
	}
}

func applyDuplicateExceptions(rows []sourceRow) {
	bodyGroups := make(map[string][]int)
	titleGroups := make(map[string][]int)
	for i := range rows {
		bodyGroups[rows[i].SourceHash] = append(bodyGroups[rows[i].SourceHash], i)
		titleGroups[strings.ToLower(rows[i].Title)] = append(titleGroups[strings.ToLower(rows[i].Title)], i)
	}
	for _, indexes := range bodyGroups {
		if len(indexes) < 2 {
			continue
		}
		sort.Slice(indexes, func(i, j int) bool { return rows[indexes[i]].Path < rows[indexes[j]].Path })
		for _, index := range indexes[1:] {
			rows[index].Exceptions = append(rows[index].Exceptions, Exception{
				Path: rows[index].Path, Kind: "dupe_exact", Resolution: ActionSkip,
				Detail: "body-identical to " + rows[indexes[0]].Path,
			})
		}
	}
	for _, indexes := range titleGroups {
		if len(indexes) < 2 {
			continue
		}
		paths := make([]string, 0, len(indexes))
		for _, index := range indexes {
			paths = append(paths, rows[index].Path)
		}
		sort.Strings(paths)
		for _, index := range indexes {
			rows[index].Exceptions = append(rows[index].Exceptions, Exception{
				Path: rows[index].Path, Kind: "dupe_title", Resolution: ActionCreate,
				Detail: "case-insensitive duplicate title: " + strings.Join(paths, ", "),
			})
		}
	}
	for i := range rows {
		sortExceptions(rows[i].Exceptions)
	}
}

func matchesAnyGlob(manifestPath string, patterns []string) bool {
	for _, pattern := range patterns {
		if globMatch(pattern, manifestPath) {
			return true
		}
	}
	return false
}

func globMatch(pattern, value string) bool {
	pattern = strings.ReplaceAll(pattern, `\`, "/")
	var builder strings.Builder
	builder.WriteByte('^')
	for i := 0; i < len(pattern); {
		if pattern[i] == '*' {
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i += 2
				if i < len(pattern) && pattern[i] == '/' {
					i++
					builder.WriteString(`(?:.*/)?`)
				} else {
					builder.WriteString(`.*`)
				}
			} else {
				i++
				builder.WriteString(`[^/]*`)
			}
			continue
		}
		if strings.ContainsRune(`.+?()|[]{}^$`, rune(pattern[i])) {
			builder.WriteByte('\\')
		}
		builder.WriteByte(pattern[i])
		i++
	}
	builder.WriteByte('$')
	matched, _ := regexp.MatchString(builder.String(), value)
	return matched
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	result := values[:0]
	var prior string
	for i, value := range values {
		if i == 0 || value != prior {
			result = append(result, value)
			prior = value
		}
	}
	return result
}
