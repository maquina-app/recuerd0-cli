package commands

import (
	"fmt"
	"net/url"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/response"
)

// workspace context — fetches the wake-up context payload for a workspace.
//
// Returns workspace metadata plus the current user's pinned memories
// (filtered to this workspace), suitable for loading into an AI agent's
// system prompt as a one-call snapshot.

var (
	workspaceContextLimit        int
	workspaceContextNoBody       bool
	workspaceContextMaxBodyChars int
)

var workspaceContextCmd = &cobra.Command{
	Use:   "context <id>",
	Short: "Fetch wake-up context (metadata + pinned memories) for a workspace",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}

		if workspaceContextLimit < 1 || workspaceContextLimit > 50 {
			exitWithError(errors.NewInvalidArgsError("--limit must be between 1 and 50"))
			return
		}
		if workspaceContextMaxBodyChars < 100 || workspaceContextMaxBodyChars > 5000 {
			exitWithError(errors.NewInvalidArgsError("--max-body-chars must be between 100 and 5000"))
			return
		}

		params := url.Values{}
		params.Set("limit", strconv.Itoa(workspaceContextLimit))
		if workspaceContextNoBody {
			params.Set("include_body", "false")
		}
		params.Set("max_body_chars", strconv.Itoa(workspaceContextMaxBodyChars))

		path := fmt.Sprintf("/workspaces/%s/context?%s", args[0], params.Encode())

		apiClient := getClient()
		resp, err := apiClient.Get(path)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("show", fmt.Sprintf("recuerd0 workspace show %s", args[0]), "View workspace details"),
			breadcrumb("list-memories", fmt.Sprintf("recuerd0 memory list --workspace %s", args[0]), "List memories in workspace"),
			breadcrumb("search", "recuerd0 search \"<query>\"", "Search across memories"),
		}

		printSuccessWithBreadcrumbs(resp.Data, "Workspace context", bc)
	},
}

func init() {
	workspaceContextCmd.Flags().IntVar(&workspaceContextLimit, "limit", 10, "max pinned memories to include (1-50)")
	workspaceContextCmd.Flags().BoolVar(&workspaceContextNoBody, "no-body", false, "exclude memory bodies (metadata only)")
	workspaceContextCmd.Flags().IntVar(&workspaceContextMaxBodyChars, "max-body-chars", 500, "truncate each body to this many characters (100-5000)")
	workspaceCmd.AddCommand(workspaceContextCmd)
}
