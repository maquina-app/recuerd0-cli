package commands

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/response"
)

var memoryReadCmd = &cobra.Command{
	Use:   "read",
	Short: "Read and slice memory content",
}

// memory read head
var (
	memoryReadHeadWorkspace string
	memoryReadHeadLines     int
)

var memoryReadHeadCmd = &cobra.Command{
	Use:   "head <memory_id>",
	Short: "Read the first N lines of a memory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryReadHeadWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}
		if memoryReadHeadLines < 1 {
			exitWithError(errors.NewInvalidArgsError("--lines must be >= 1"))
			return
		}

		path := fmt.Sprintf("/workspaces/%s/memories/%s?line_start=1&line_end=%d", ws, args[0], memoryReadHeadLines)
		apiClient := getClient()
		resp, err := apiClient.Get(path)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("tail", fmt.Sprintf("recuerd0 memory read tail --workspace %s %s", ws, args[0]), "Read the last lines"),
			breadcrumb("lines", fmt.Sprintf("recuerd0 memory read lines --workspace %s %s --start 1 --end 100", ws, args[0]), "Read a specific window"),
		}
		printSuccessWithBreadcrumbs(resp.Data, "Memory head", bc)
	},
}

// memory read tail
var (
	memoryReadTailWorkspace string
	memoryReadTailLines     int
)

var memoryReadTailCmd = &cobra.Command{
	Use:   "tail <memory_id>",
	Short: "Read the last N lines of a memory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryReadTailWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}
		if memoryReadTailLines < 1 {
			exitWithError(errors.NewInvalidArgsError("--lines must be >= 1"))
			return
		}

		apiClient := getClient()

		// Step 1: cheap call to learn total_lines.
		probePath := fmt.Sprintf("/workspaces/%s/memories/%s?line_start=1&line_end=1", ws, args[0])
		probe, err := apiClient.Get(probePath)
		if err != nil {
			exitWithError(err)
			return
		}

		total, err := extractTotalLines(probe.Data)
		if err != nil {
			exitWithError(errors.NewError(fmt.Sprintf("could not determine total_lines: %v", err)))
			return
		}

		start := total - memoryReadTailLines + 1
		if start < 1 {
			start = 1
		}

		path := fmt.Sprintf("/workspaces/%s/memories/%s?line_start=%d&line_end=%d", ws, args[0], start, total)
		resp, err := apiClient.Get(path)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("head", fmt.Sprintf("recuerd0 memory read head --workspace %s %s", ws, args[0]), "Read the first lines"),
			breadcrumb("lines", fmt.Sprintf("recuerd0 memory read lines --workspace %s %s --start 1 --end %d", ws, args[0], total), "Read a specific window"),
		}
		printSuccessWithBreadcrumbs(resp.Data, "Memory tail", bc)
	},
}

// extractTotalLines pulls content.total_lines out of an API response payload.
func extractTotalLines(data interface{}) (int, error) {
	m, ok := data.(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("unexpected response shape")
	}
	content, ok := m["content"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("missing content object")
	}
	switch v := content["total_lines"].(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, err
		}
		return n, nil
	default:
		return 0, fmt.Errorf("total_lines missing or of unexpected type")
	}
}

// memory read lines
var (
	memoryReadLinesWorkspace string
	memoryReadLinesStart     int
	memoryReadLinesEnd       int
)

var memoryReadLinesCmd = &cobra.Command{
	Use:   "lines <memory_id>",
	Short: "Read a specific line window of a memory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryReadLinesWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}
		if memoryReadLinesStart < 1 {
			exitWithError(errors.NewInvalidArgsError("--start must be >= 1"))
			return
		}
		if memoryReadLinesEnd < memoryReadLinesStart {
			exitWithError(errors.NewInvalidArgsError("--end must be >= --start"))
			return
		}

		path := fmt.Sprintf("/workspaces/%s/memories/%s?line_start=%d&line_end=%d", ws, args[0], memoryReadLinesStart, memoryReadLinesEnd)
		apiClient := getClient()
		resp, err := apiClient.Get(path)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("expand", fmt.Sprintf("recuerd0 memory read lines --workspace %s %s --start %d --end %d", ws, args[0], memoryReadLinesStart, memoryReadLinesEnd+50), "Expand the window"),
			breadcrumb("grep", fmt.Sprintf("recuerd0 memory read grep --workspace %s %s PATTERN", ws, args[0]), "Search inside the memory"),
		}
		printSuccessWithBreadcrumbs(resp.Data, "Memory lines", bc)
	},
}

// memory read grep
var (
	memoryReadGrepWorkspace string
	memoryReadGrepContext   int
	memoryReadGrepBefore    int
	memoryReadGrepAfter     int
)

var memoryReadGrepCmd = &cobra.Command{
	Use:   "grep <memory_id> <pattern>",
	Short: "Grep memory content with optional context lines",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryReadGrepWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}
		if memoryReadGrepContext < 0 || memoryReadGrepContext > 10 {
			exitWithError(errors.NewInvalidArgsError("--context must be between 0 and 10"))
			return
		}
		if memoryReadGrepBefore != -1 && (memoryReadGrepBefore < 0 || memoryReadGrepBefore > 10) {
			exitWithError(errors.NewInvalidArgsError("--before must be between 0 and 10"))
			return
		}
		if memoryReadGrepAfter != -1 && (memoryReadGrepAfter < 0 || memoryReadGrepAfter > 10) {
			exitWithError(errors.NewInvalidArgsError("--after must be between 0 and 10"))
			return
		}

		pattern := args[1]
		q := url.QueryEscape(pattern)
		path := fmt.Sprintf("/workspaces/%s/memories/%s?mode=grep&q=%s", ws, args[0], q)
		if memoryReadGrepContext > 0 {
			path += fmt.Sprintf("&context=%d", memoryReadGrepContext)
		}
		if memoryReadGrepBefore != -1 {
			path += fmt.Sprintf("&before=%d", memoryReadGrepBefore)
		}
		if memoryReadGrepAfter != -1 {
			path += fmt.Sprintf("&after=%d", memoryReadGrepAfter)
		}

		apiClient := getClient()
		resp, err := apiClient.Get(path)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := buildGrepBreadcrumbs(resp.Data, ws, args[0])
		printSuccessWithBreadcrumbs(resp.Data, "Memory grep", bc)
	},
}

// buildGrepBreadcrumbs emits one "memory read lines" suggestion per match,
// capped at the first 5, with a -2/+5 window around each match line.
func buildGrepBreadcrumbs(data interface{}, ws, memoryID string) []response.Breadcrumb {
	bc := []response.Breadcrumb{}
	m, ok := data.(map[string]interface{})
	if !ok {
		return bc
	}
	content, ok := m["content"].(map[string]interface{})
	if !ok {
		return bc
	}
	matches, ok := content["matches"].([]interface{})
	if !ok {
		return bc
	}
	limit := len(matches)
	if limit > 5 {
		limit = 5
	}
	for i := 0; i < limit; i++ {
		entry, ok := matches[i].(map[string]interface{})
		if !ok {
			continue
		}
		var ln int
		switch v := entry["line_number"].(type) {
		case float64:
			ln = int(v)
		case int:
			ln = v
		default:
			continue
		}
		start := ln - 2
		if start < 1 {
			start = 1
		}
		end := ln + 5
		bc = append(bc, breadcrumb(
			"lines",
			fmt.Sprintf("recuerd0 memory read lines --workspace %s %s --start %d --end %d", ws, memoryID, start, end),
			fmt.Sprintf("Read window around match on line %d", ln),
		))
	}
	return bc
}

func init() {
	memoryReadHeadCmd.Flags().StringVar(&memoryReadHeadWorkspace, "workspace", "", "workspace ID")
	memoryReadHeadCmd.Flags().IntVar(&memoryReadHeadLines, "lines", 20, "number of lines to read from the top")
	memoryReadCmd.AddCommand(memoryReadHeadCmd)

	memoryReadTailCmd.Flags().StringVar(&memoryReadTailWorkspace, "workspace", "", "workspace ID")
	memoryReadTailCmd.Flags().IntVar(&memoryReadTailLines, "lines", 20, "number of lines to read from the bottom")
	memoryReadCmd.AddCommand(memoryReadTailCmd)

	memoryReadLinesCmd.Flags().StringVar(&memoryReadLinesWorkspace, "workspace", "", "workspace ID")
	memoryReadLinesCmd.Flags().IntVar(&memoryReadLinesStart, "start", 0, "first line (1-based, inclusive) (required)")
	memoryReadLinesCmd.Flags().IntVar(&memoryReadLinesEnd, "end", 0, "last line (1-based, inclusive) (required)")
	_ = memoryReadLinesCmd.MarkFlagRequired("start")
	_ = memoryReadLinesCmd.MarkFlagRequired("end")
	memoryReadCmd.AddCommand(memoryReadLinesCmd)

	memoryReadGrepCmd.Flags().StringVar(&memoryReadGrepWorkspace, "workspace", "", "workspace ID")
	memoryReadGrepCmd.Flags().IntVar(&memoryReadGrepContext, "context", 0, "lines of context before and after each match (0-10)")
	memoryReadGrepCmd.Flags().IntVar(&memoryReadGrepBefore, "before", -1, "lines of context before each match (0-10, overrides --context)")
	memoryReadGrepCmd.Flags().IntVar(&memoryReadGrepAfter, "after", -1, "lines of context after each match (0-10, overrides --context)")
	memoryReadCmd.AddCommand(memoryReadGrepCmd)

	memoryCmd.AddCommand(memoryReadCmd)
}
