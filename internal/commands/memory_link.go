package commands

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/response"
)

var memoryLinkCmd = &cobra.Command{
	Use:   "link",
	Short: "Manage cross-workspace memory links",
}

// memory link list
var memoryLinkListWorkspace string

var memoryLinkListCmd = &cobra.Command{
	Use:   "list <memory_id>",
	Short: "List linked memories for a memory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryLinkListWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}
		memID, err := strconv.Atoi(args[0])
		if err != nil || memID <= 0 {
			exitWithError(errors.NewInvalidArgsError("memory_id must be a positive integer"))
			return
		}

		path := fmt.Sprintf("/workspaces/%s/memories/%d/links", ws, memID)
		apiClient := getClient()
		resp, err := apiClient.Get(path)
		if err != nil {
			exitWithError(err)
			return
		}

		items := countItems(resp.Data)
		summary := fmt.Sprintf("%d link(s)", items)
		bc := []response.Breadcrumb{
			breadcrumb("add", fmt.Sprintf("recuerd0 memory link add %d --to <other_id>", memID), "Link to another memory"),
			breadcrumb("show", fmt.Sprintf("recuerd0 memory show --workspace %s %d", ws, memID), "View memory details"),
		}
		printSuccessWithBreadcrumbs(resp.Data, summary, bc)
	},
}

// memory link add
var (
	memoryLinkAddWorkspace string
	memoryLinkAddTo        int
)

var memoryLinkAddCmd = &cobra.Command{
	Use:   "add <memory_id>",
	Short: "Link this memory to another memory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryLinkAddWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}
		memID, err := strconv.Atoi(args[0])
		if err != nil || memID <= 0 {
			exitWithError(errors.NewInvalidArgsError("memory_id must be a positive integer"))
			return
		}
		if memoryLinkAddTo <= 0 {
			exitWithError(errors.NewInvalidArgsError("--to must be a positive integer"))
			return
		}
		if memoryLinkAddTo == memID {
			exitWithError(errors.NewInvalidArgsError("cannot link a memory to itself"))
			return
		}

		body := map[string]interface{}{"to_memory_id": memoryLinkAddTo}
		path := fmt.Sprintf("/workspaces/%s/memories/%d/links", ws, memID)
		apiClient := getClient()
		resp, err := apiClient.Post(path, body)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("list", fmt.Sprintf("recuerd0 memory link list %d", memID), "List all links"),
			breadcrumb("remove", fmt.Sprintf("recuerd0 memory link remove %d --to %d", memID, memoryLinkAddTo), "Remove this link"),
		}
		printSuccessWithBreadcrumbs(resp.Data, "Memory link created", bc)
	},
}

// memory link remove
var (
	memoryLinkRemoveWorkspace string
	memoryLinkRemoveTo        int
)

var memoryLinkRemoveCmd = &cobra.Command{
	Use:   "remove <memory_id>",
	Short: "Remove a link between two memories",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryLinkRemoveWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}
		memID, err := strconv.Atoi(args[0])
		if err != nil || memID <= 0 {
			exitWithError(errors.NewInvalidArgsError("memory_id must be a positive integer"))
			return
		}
		if memoryLinkRemoveTo <= 0 {
			exitWithError(errors.NewInvalidArgsError("--to must be a positive integer"))
			return
		}
		if memoryLinkRemoveTo == memID {
			exitWithError(errors.NewInvalidArgsError("--to cannot equal memory_id"))
			return
		}

		path := fmt.Sprintf("/workspaces/%s/memories/%d/links/%d", ws, memID, memoryLinkRemoveTo)
		apiClient := getClient()
		_, err = apiClient.Delete(path)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("list", fmt.Sprintf("recuerd0 memory link list %d", memID), "List remaining links"),
		}
		printSuccessWithBreadcrumbs(nil, "Memory link removed", bc)
	},
}

func init() {
	memoryLinkListCmd.Flags().StringVar(&memoryLinkListWorkspace, "workspace", "", "workspace ID")
	memoryLinkAddCmd.Flags().StringVar(&memoryLinkAddWorkspace, "workspace", "", "workspace ID")
	memoryLinkAddCmd.Flags().IntVar(&memoryLinkAddTo, "to", 0, "id of the memory to link to (required)")
	memoryLinkRemoveCmd.Flags().StringVar(&memoryLinkRemoveWorkspace, "workspace", "", "workspace ID")
	memoryLinkRemoveCmd.Flags().IntVar(&memoryLinkRemoveTo, "to", 0, "id of the memory the link points to (required)")

	memoryLinkCmd.AddCommand(memoryLinkListCmd)
	memoryLinkCmd.AddCommand(memoryLinkAddCmd)
	memoryLinkCmd.AddCommand(memoryLinkRemoveCmd)
	memoryCmd.AddCommand(memoryLinkCmd)
}
