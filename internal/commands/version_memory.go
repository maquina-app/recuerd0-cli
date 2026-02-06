package commands

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/response"
)

var memoryVersionCmd = &cobra.Command{
	Use:   "version",
	Short: "Manage memory versions",
}

// memory version create
var (
	memoryVersionCreateWorkspace string
	memoryVersionCreateTitle     string
	memoryVersionCreateContent   string
	memoryVersionCreateSource    string
	memoryVersionCreateTags      string
)

var memoryVersionCreateCmd = &cobra.Command{
	Use:   "create <memory_id>",
	Short: "Create a new version of a memory",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		ws, err := resolveWorkspace(memoryVersionCreateWorkspace)
		if err != nil {
			exitWithError(err)
			return
		}

		content := memoryVersionCreateContent
		if content == "-" {
			data, err := io.ReadAll(stdinReader())
			if err != nil {
				exitWithError(errors.NewError(fmt.Sprintf("reading stdin: %v", err)))
				return
			}
			content = string(data)
		}

		version := map[string]interface{}{}
		if memoryVersionCreateTitle != "" {
			version["title"] = memoryVersionCreateTitle
		}
		if content != "" {
			version["content"] = content
		}
		if memoryVersionCreateSource != "" {
			version["source"] = memoryVersionCreateSource
		}
		if memoryVersionCreateTags != "" {
			tags := strings.Split(memoryVersionCreateTags, ",")
			trimmed := make([]string, 0, len(tags))
			for _, t := range tags {
				t = strings.TrimSpace(t)
				if t != "" {
					trimmed = append(trimmed, t)
				}
			}
			version["tags"] = trimmed
		}

		body := map[string]interface{}{"version": version}

		apiClient := getClient()
		resp, err := apiClient.Post(fmt.Sprintf("/api/v1/workspaces/%s/memories/%s/versions", ws, args[0]), body)
		if err != nil {
			exitWithError(err)
			return
		}

		bc := []response.Breadcrumb{
			breadcrumb("show", fmt.Sprintf("recuerd0 memory show --workspace %s %s", ws, args[0]), "View memory"),
			breadcrumb("list", fmt.Sprintf("recuerd0 memory list --workspace %s", ws), "List memories"),
		}

		printSuccessWithBreadcrumbs(resp.Data, "Version created", bc)
	},
}

func init() {
	memoryCmd.AddCommand(memoryVersionCmd)

	memoryVersionCreateCmd.Flags().StringVar(&memoryVersionCreateWorkspace, "workspace", "", "workspace ID")
	memoryVersionCreateCmd.Flags().StringVar(&memoryVersionCreateTitle, "title", "", "version title")
	memoryVersionCreateCmd.Flags().StringVar(&memoryVersionCreateContent, "content", "", "version content (use - for stdin)")
	memoryVersionCreateCmd.Flags().StringVar(&memoryVersionCreateSource, "source", "", "source")
	memoryVersionCreateCmd.Flags().StringVar(&memoryVersionCreateTags, "tags", "", "comma-separated tags")
	memoryVersionCmd.AddCommand(memoryVersionCreateCmd)
}
