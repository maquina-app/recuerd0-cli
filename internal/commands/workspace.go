package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/response"
)

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces",
}

// workspace list
var workspaceListPage string

var workspaceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List workspaces",
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}

		path := "/workspaces"
		if workspaceListPage != "" {
			path += "?page=" + workspaceListPage
		}

		apiClient := getClient()
		resp, err := apiClient.GetWithPagination(path)
		if err != nil {
			exitWithError(err)
			return
		}

		hasNext := resp.LinkNext != ""
		items := countItems(resp.Data)
		summary := fmt.Sprintf("%d workspace(s)", items)

		bc := []response.Breadcrumb{
			breadcrumb("show", "recuerd0 workspace show <id>", "View workspace details"),
			breadcrumb("create", "recuerd0 workspace create --name NAME", "Create a workspace"),
		}

		printSuccessWithPaginationAndBreadcrumbs(resp.Data, hasNext, resp.LinkNext, summary, bc)
	},
}

// workspace show
var workspaceShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show workspace details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}

		apiClient := getClient()
		resp, err := apiClient.Get("/workspaces/" + args[0])
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("list-memories", fmt.Sprintf("recuerd0 memory list --workspace %s", args[0]), "List memories in workspace"),
			breadcrumb("update", fmt.Sprintf("recuerd0 workspace update %s --name NAME", args[0]), "Update workspace"),
			breadcrumb("archive", fmt.Sprintf("recuerd0 workspace archive %s", args[0]), "Archive workspace"),
		}

		printSuccessWithBreadcrumbs(resp.Data, "Workspace details", bc)
	},
}

// workspace create
var (
	workspaceCreateName string
	workspaceCreateDesc string
)

var workspaceCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new workspace",
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		if workspaceCreateName == "" {
			exitWithError(errors.NewInvalidArgsError("--name is required"))
			return
		}

		body := map[string]interface{}{
			"workspace": map[string]interface{}{
				"name":        workspaceCreateName,
				"description": workspaceCreateDesc,
			},
		}

		apiClient := getClient()
		resp, err := apiClient.Post("/workspaces", body)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("show", "recuerd0 workspace show <id>", "View created workspace"),
			breadcrumb("list", "recuerd0 workspace list", "List all workspaces"),
		}

		printSuccessWithBreadcrumbs(resp.Data, "Workspace created", bc)
	},
}

// workspace update
var (
	workspaceUpdateName string
	workspaceUpdateDesc string
)

var workspaceUpdateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update a workspace",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}

		workspace := map[string]interface{}{}
		if workspaceUpdateName != "" {
			workspace["name"] = workspaceUpdateName
		}
		if workspaceUpdateDesc != "" {
			workspace["description"] = workspaceUpdateDesc
		}
		if len(workspace) == 0 {
			exitWithError(errors.NewInvalidArgsError("at least one of --name or --description is required"))
			return
		}

		body := map[string]interface{}{"workspace": workspace}

		apiClient := getClient()
		resp, err := apiClient.Patch("/workspaces/"+args[0], body)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("show", fmt.Sprintf("recuerd0 workspace show %s", args[0]), "View updated workspace"),
		}

		printSuccessWithBreadcrumbs(resp.Data, "Workspace updated", bc)
	},
}

func countItems(data interface{}) int {
	if arr, ok := data.([]interface{}); ok {
		return len(arr)
	}
	return 0
}

func countSearchResults(data interface{}) int {
	if m, ok := data.(map[string]interface{}); ok {
		if total, ok := m["total_results"].(float64); ok {
			return int(total)
		}
		if results, ok := m["results"].([]interface{}); ok {
			return len(results)
		}
	}
	return 0
}

func init() {
	rootCmd.AddCommand(workspaceCmd)

	workspaceListCmd.Flags().StringVar(&workspaceListPage, "page", "", "page number")
	workspaceCmd.AddCommand(workspaceListCmd)

	workspaceCmd.AddCommand(workspaceShowCmd)

	workspaceCreateCmd.Flags().StringVar(&workspaceCreateName, "name", "", "workspace name (required)")
	workspaceCreateCmd.Flags().StringVar(&workspaceCreateDesc, "description", "", "workspace description")
	workspaceCmd.AddCommand(workspaceCreateCmd)

	workspaceUpdateCmd.Flags().StringVar(&workspaceUpdateName, "name", "", "workspace name")
	workspaceUpdateCmd.Flags().StringVar(&workspaceUpdateDesc, "description", "", "workspace description")
	workspaceCmd.AddCommand(workspaceUpdateCmd)
}
