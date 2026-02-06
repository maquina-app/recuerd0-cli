package commands

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/response"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage memories",
}

// resolveWorkspace gets workspace from flag or config.
func resolveWorkspace(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	return requireWorkspace()
}

// memory list
var (
	memoryListWorkspace string
	memoryListPage      string
)

var memoryListCmd = &cobra.Command{
	Use:   "list",
	Short: "List memories in a workspace",
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryListWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}

		path := fmt.Sprintf("/api/v1/workspaces/%s/memories", ws)
		if memoryListPage != "" {
			path += "?page=" + memoryListPage
		}

		apiClient := getClient()
		resp, err := apiClient.GetWithPagination(path)
		if err != nil {
			exitWithError(err)
			return
		}

		hasNext := resp.LinkNext != ""
		items := countItems(resp.Data)
		summary := fmt.Sprintf("%d memory(ies)", items)

		bc := []response.Breadcrumb{
			breadcrumb("show", fmt.Sprintf("recuerd0 memory show --workspace %s <memory_id>", ws), "View memory details"),
			breadcrumb("create", fmt.Sprintf("recuerd0 memory create --workspace %s --title TITLE --content CONTENT", ws), "Create a memory"),
		}

		printSuccessWithPaginationAndBreadcrumbs(resp.Data, hasNext, resp.LinkNext, summary, bc)
	},
}

// memory show
var memoryShowWorkspace string

var memoryShowCmd = &cobra.Command{
	Use:   "show <memory_id>",
	Short: "Show memory details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryShowWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}

		apiClient := getClient()
		resp, err := apiClient.Get(fmt.Sprintf("/api/v1/workspaces/%s/memories/%s", ws, args[0]))
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("update", fmt.Sprintf("recuerd0 memory update --workspace %s %s --title TITLE", ws, args[0]), "Update memory"),
			breadcrumb("version", fmt.Sprintf("recuerd0 memory version create --workspace %s %s", ws, args[0]), "Create a version"),
			breadcrumb("delete", fmt.Sprintf("recuerd0 memory delete --workspace %s %s", ws, args[0]), "Delete memory"),
		}

		printSuccessWithBreadcrumbs(resp.Data, "Memory details", bc)
	},
}

// memory create
var (
	memoryCreateWorkspace string
	memoryCreateTitle     string
	memoryCreateContent   string
	memoryCreateSource    string
	memoryCreateTags      string
)

var memoryCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new memory",
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryCreateWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}

		content := memoryCreateContent
		if content == "-" {
			data, err := io.ReadAll(stdinReader())
			if err != nil {
				exitWithError(errors.NewError(fmt.Sprintf("reading stdin: %v", err)))
				return
			}
			content = string(data)
		}

		memory := map[string]interface{}{}
		if memoryCreateTitle != "" {
			memory["title"] = memoryCreateTitle
		}
		if content != "" {
			memory["content"] = content
		}
		if memoryCreateSource != "" {
			memory["source"] = memoryCreateSource
		}
		if memoryCreateTags != "" {
			memory["tags"] = parseTags(memoryCreateTags)
		}

		body := map[string]interface{}{"memory": memory}

		apiClient := getClient()
		resp, err := apiClient.Post(fmt.Sprintf("/api/v1/workspaces/%s/memories", ws), body)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("show", fmt.Sprintf("recuerd0 memory show --workspace %s <memory_id>", ws), "View created memory"),
			breadcrumb("list", fmt.Sprintf("recuerd0 memory list --workspace %s", ws), "List all memories"),
		}

		printSuccessWithBreadcrumbs(resp.Data, "Memory created", bc)
	},
}

// memory update
var (
	memoryUpdateWorkspace string
	memoryUpdateTitle     string
	memoryUpdateContent   string
	memoryUpdateSource    string
	memoryUpdateTags      string
)

var memoryUpdateCmd = &cobra.Command{
	Use:   "update <memory_id>",
	Short: "Update an existing memory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryUpdateWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}

		content := memoryUpdateContent
		if content == "-" {
			data, err := io.ReadAll(stdinReader())
			if err != nil {
				exitWithError(errors.NewError(fmt.Sprintf("reading stdin: %v", err)))
				return
			}
			content = string(data)
		}

		memory := map[string]interface{}{}
		if memoryUpdateTitle != "" {
			memory["title"] = memoryUpdateTitle
		}
		if content != "" {
			memory["content"] = content
		}
		if memoryUpdateSource != "" {
			memory["source"] = memoryUpdateSource
		}
		if memoryUpdateTags != "" {
			memory["tags"] = parseTags(memoryUpdateTags)
		}

		if len(memory) == 0 {
			exitWithError(errors.NewInvalidArgsError("at least one field to update is required"))
			return
		}

		body := map[string]interface{}{"memory": memory}

		apiClient := getClient()
		resp, err := apiClient.Patch(fmt.Sprintf("/api/v1/workspaces/%s/memories/%s", ws, args[0]), body)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("show", fmt.Sprintf("recuerd0 memory show --workspace %s %s", ws, args[0]), "View updated memory"),
		}

		printSuccessWithBreadcrumbs(resp.Data, "Memory updated", bc)
	},
}

// memory delete
var memoryDeleteWorkspace string

var memoryDeleteCmd = &cobra.Command{
	Use:   "delete <memory_id>",
	Short: "Delete a memory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryDeleteWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}

		apiClient := getClient()
		_, err = apiClient.Delete(fmt.Sprintf("/api/v1/workspaces/%s/memories/%s", ws, args[0]))
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("list", fmt.Sprintf("recuerd0 memory list --workspace %s", ws), "List remaining memories"),
		}

		printSuccessWithBreadcrumbs(
			map[string]string{"deleted": args[0]},
			fmt.Sprintf("Memory %s deleted", args[0]),
			bc,
		)
	},
}

func parseTags(s string) []string {
	parts := strings.Split(s, ",")
	tags := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tags = append(tags, p)
		}
	}
	return tags
}

// stdinReader returns os.Stdin, overridable for tests.
var stdinReader = func() io.Reader {
	return os.Stdin
}

func init() {
	rootCmd.AddCommand(memoryCmd)

	memoryListCmd.Flags().StringVar(&memoryListWorkspace, "workspace", "", "workspace ID")
	memoryListCmd.Flags().StringVar(&memoryListPage, "page", "", "page number")
	memoryCmd.AddCommand(memoryListCmd)

	memoryShowCmd.Flags().StringVar(&memoryShowWorkspace, "workspace", "", "workspace ID")
	memoryCmd.AddCommand(memoryShowCmd)

	memoryCreateCmd.Flags().StringVar(&memoryCreateWorkspace, "workspace", "", "workspace ID")
	memoryCreateCmd.Flags().StringVar(&memoryCreateTitle, "title", "", "memory title")
	memoryCreateCmd.Flags().StringVar(&memoryCreateContent, "content", "", "memory content (use - for stdin)")
	memoryCreateCmd.Flags().StringVar(&memoryCreateSource, "source", "", "source of the memory")
	memoryCreateCmd.Flags().StringVar(&memoryCreateTags, "tags", "", "comma-separated tags")
	memoryCmd.AddCommand(memoryCreateCmd)

	memoryUpdateCmd.Flags().StringVar(&memoryUpdateWorkspace, "workspace", "", "workspace ID")
	memoryUpdateCmd.Flags().StringVar(&memoryUpdateTitle, "title", "", "memory title")
	memoryUpdateCmd.Flags().StringVar(&memoryUpdateContent, "content", "", "memory content (use - for stdin)")
	memoryUpdateCmd.Flags().StringVar(&memoryUpdateSource, "source", "", "source of the memory")
	memoryUpdateCmd.Flags().StringVar(&memoryUpdateTags, "tags", "", "comma-separated tags")
	memoryCmd.AddCommand(memoryUpdateCmd)

	memoryDeleteCmd.Flags().StringVar(&memoryDeleteWorkspace, "workspace", "", "workspace ID")
	memoryCmd.AddCommand(memoryDeleteCmd)
}
