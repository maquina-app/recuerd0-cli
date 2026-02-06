package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/response"
)

var workspaceArchiveCmd = &cobra.Command{
	Use:   "archive <id>",
	Short: "Archive a workspace",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}

		apiClient := getClient()
		resp, err := apiClient.Patch("/api/v1/workspaces/"+args[0]+"/archive", nil)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("unarchive", fmt.Sprintf("recuerd0 workspace unarchive %s", args[0]), "Unarchive workspace"),
			breadcrumb("list", "recuerd0 workspace list", "List workspaces"),
		}

		printSuccessWithBreadcrumbs(resp.Data, "Workspace archived", bc)
	},
}

var workspaceUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <id>",
	Short: "Unarchive a workspace",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}

		apiClient := getClient()
		resp, err := apiClient.Patch("/api/v1/workspaces/"+args[0]+"/unarchive", nil)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("show", fmt.Sprintf("recuerd0 workspace show %s", args[0]), "View workspace"),
			breadcrumb("list", "recuerd0 workspace list", "List workspaces"),
		}

		printSuccessWithBreadcrumbs(resp.Data, "Workspace unarchived", bc)
	},
}

func init() {
	workspaceCmd.AddCommand(workspaceArchiveCmd)
	workspaceCmd.AddCommand(workspaceUnarchiveCmd)
}
