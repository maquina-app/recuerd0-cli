package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/response"
)

var (
	searchWorkspace string
	searchPage      string
)

var searchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Search memories",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}

		query := args[0]
		if query == "" {
			exitWithError(errors.NewInvalidArgsError("search query is required"))
			return
		}

		path := "/search?q=" + query
		if searchWorkspace != "" {
			path += "&workspace_id=" + searchWorkspace
		}
		if searchPage != "" {
			path += "&page=" + searchPage
		}

		apiClient := getClient()
		resp, err := apiClient.GetWithPagination(path)
		if err != nil {
			exitWithError(err)
			return
		}

		hasNext := resp.LinkNext != ""
		items := countSearchResults(resp.Data)
		summary := fmt.Sprintf("%d result(s) for %q", items, query)

		bc := []response.Breadcrumb{
			breadcrumb("show", "recuerd0 memory show --workspace <id> <memory_id>", "View memory details"),
		}

		printSuccessWithPaginationAndBreadcrumbs(resp.Data, hasNext, resp.LinkNext, summary, bc)
	},
}

func init() {
	searchCmd.Flags().StringVar(&searchWorkspace, "workspace", "", "limit search to workspace")
	searchCmd.Flags().StringVar(&searchPage, "page", "", "page number")
	rootCmd.AddCommand(searchCmd)
}
