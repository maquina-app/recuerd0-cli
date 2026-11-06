package commands

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/response"
)

var (
	searchWorkspace string
	searchPage      string
	searchCategory  string
	searchInclude   string
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

		if !IsValidCategory(searchCategory) {
			exitWithError(errors.NewInvalidArgsError("--category must be one of: decision, discovery, preference, general"))
			return
		}

		// An --include token we do not recognize widens retrieval by a name that
		// matches nothing. Answer with an empty result set rather than spending a
		// request that would return the unwidened list.
		if !IsValidInclude(searchInclude) {
			printSuccessWithBreadcrumbs([]interface{}{}, unknownIncludeSummary(searchInclude), searchBreadcrumbs())
			return
		}

		params := url.Values{}
		params.Set("q", query)
		if searchWorkspace != "" {
			params.Set("workspace_id", searchWorkspace)
		}
		if searchPage != "" {
			params.Set("page", searchPage)
		}
		if searchCategory != "" {
			params.Set("category", searchCategory)
		}
		if inc := includeParam(searchInclude); inc != "" {
			params.Set("include", inc)
		}
		path := "/search?" + params.Encode()

		apiClient := getClient()
		resp, err := apiClient.GetWithPagination(path)
		if err != nil {
			exitWithError(err)
			return
		}

		hasNext := resp.LinkNext != ""
		items := countSearchResults(resp.Data)
		summary := fmt.Sprintf("%d result(s) for %q", items, query)

		bc := searchBreadcrumbs()

		// Otherwise "to in" reads as nothing matching, not as nothing searched.
		if ignored := ignoredWords(resp.Data); len(ignored) > 0 {
			summary += fmt.Sprintf(" (ignored words under 3 characters: %s)", strings.Join(ignored, ", "))
		}

		// The server withholds memories tagged obsolete/superseded/deprecated and
		// reports the count in obsolete_hidden. Say so, and offer the way back —
		// otherwise a caller auditing retired knowledge never learns it exists.
		if hidden := countObsoleteHidden(resp.Data); hidden > 0 {
			summary += fmt.Sprintf(" (%d obsolete hidden)", hidden)
			bc = append(bc, breadcrumb(
				"include-obsolete",
				fmt.Sprintf("recuerd0 search %q --include obsolete", query),
				"Repeat the search including retired memories",
			))
		}

		printSuccessWithPaginationAndBreadcrumbs(resp.Data, hasNext, resp.LinkNext, summary, bc)
	},
}

func searchBreadcrumbs() []response.Breadcrumb {
	return []response.Breadcrumb{
		breadcrumb("show", "recuerd0 memory show --workspace <id> <memory_id>", "View memory details"),
	}
}

func init() {
	searchCmd.Flags().StringVar(&searchWorkspace, "workspace", "", "limit search to workspace")
	searchCmd.Flags().StringVar(&searchPage, "page", "", "page number")
	searchCmd.Flags().StringVar(&searchCategory, "category", "", "filter by category (decision, discovery, preference, general)")
	searchCmd.Flags().StringVar(&searchInclude, "include", "", "include otherwise-hidden memories (obsolete)")
	rootCmd.AddCommand(searchCmd)
}
