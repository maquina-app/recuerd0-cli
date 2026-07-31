package commands

import (
	"bufio"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/maquina/recuerd0-cli/internal/client"
	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/importer"
	"github.com/maquina/recuerd0-cli/internal/response"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Plan and execute a resumable knowledge import",
	Long:  "Import is propose → review → commit: `propose` writes a reviewable `import.plan.yaml` and never touches the server; `commit` executes the plan you approved.",
}

var (
	importProposeWorkspace string
	importProposePlan      string
	importProposeLedger    string
	importProposeAdapter   string
	importProposeFresh     bool
	importGuidanceOutput   io.Writer = os.Stderr
	importGuidanceIsTTY              = func() bool {
		return term.IsTerminal(int(os.Stderr.Fd()))
	}
	importCommitInput      io.Reader = os.Stdin
	importCommitInputIsTTY           = func() bool {
		return term.IsTerminal(int(os.Stdin.Fd()))
	}
)

const importProposeGuidance = `Plan written to %s

Review it — the manifest lists one entry per file with the title, category and
tags it will use. Edit the file if something is wrong.

  recuerd0 import commit %s    execute after review
`

const importCommitFailureGuidance = `Import stopped — nothing is lost. Anything already committed is recorded in the ledger:
  %s

Resume with: recuerd0 import commit %s
Completed rows are skipped automatically on re-run.
`

const (
	importNoWritesNotice  = "Import plan contains no writes. Nothing was written."
	importCancelledNotice = "Import cancelled. Nothing was written."
)

var importProposeCmd = &cobra.Command{
	Use:   "propose <path>",
	Short: "Scan source and atomically write a reviewable import plan",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		workspace, err := strconv.ParseInt(importProposeWorkspace, 10, 64)
		if err != nil || workspace <= 0 {
			exitWithError(errors.NewInvalidArgsError("--workspace must be a positive integer"))
			return
		}
		planPath, err := filepath.Abs(filepath.Clean(importProposePlan))
		if err != nil {
			exitWithError(errors.NewError(fmt.Sprintf("resolve plan path: %v", err)))
			return
		}
		_, digest, err := importer.Propose(getClient(), importer.ProposeOptions{
			SourcePath: args[0], PlanPath: planPath,
			LedgerPath: importProposeLedger, Adapter: importProposeAdapter,
			Workspace: workspace, Fresh: importProposeFresh,
		})
		if err != nil {
			exitWithError(importCommandError(err))
			return
		}
		displayPlanPath := displayPath(planPath)
		printImportGuidance(displayPlanPath)
		printSuccessWithDetails(
			digest,
			fmt.Sprintf("Import plan written to %s", displayPlanPath),
			displayPlanPath,
			[]response.Breadcrumb{
				breadcrumb("review-plan", displayPlanPath, "Open and review the import plan"),
				breadcrumb("commit-plan", fmt.Sprintf("recuerd0 import commit %s", displayPlanPath), "Commit the reviewed import plan"),
			},
		)
	},
}

var (
	importCommitYes    bool
	importCommitLedger string
)

var importCommitCmd = &cobra.Command{
	Use:   "commit <plan>",
	Short: "Validate and execute an approved import plan",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		options := importer.CommitOptions{PlanPath: args[0], LedgerPath: importCommitLedger}
		plan, digest, _, err := importer.Review(options)
		if err != nil {
			exitWithError(importCommandError(err))
			return
		}
		if digest.Counts.Create == 0 && digest.Counts.Version == 0 {
			fmt.Fprintln(importGuidanceOutput, importNoWritesNotice)
			return
		}
		if !importCommitYes && !importCommitInputIsTTY() {
			exitWithError(errors.NewInvalidArgsError("--yes is required when not interactive"))
			return
		}
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}

		apiClient := getClient()
		workspaceID := plan.Workspace
		workspaceName, workspaceURL, found := fetchImportWorkspace(apiClient, workspaceID)
		if !found {
			fmt.Fprintf(
				importGuidanceOutput,
				"Could not fetch the workspace name; continuing with workspace %d.\n",
				workspaceID,
			)
		}

		if !importCommitYes && !confirmImport(
			digest.Counts.Create,
			digest.Counts.Version,
			workspaceID,
			workspaceName,
		) {
			fmt.Fprintln(importGuidanceOutput, importCancelledNotice)
			return
		}

		commitSummary, err := importer.Commit(apiClient, options)
		if err != nil {
			printImportCommitFailureGuidance(commitSummary.LedgerPath, args[0])
			exitWithErrorAndData(importCommandError(err), commitSummary)
			return
		}

		summary := importCommitSummary(
			commitSummary.Ops.Created,
			commitSummary.Ops.Versioned,
			workspaceID,
			workspaceName,
		)
		afterImportPrompt := fmt.Sprintf(
			"I just imported notes into recuerd0 workspace %d. Do the after-import pass.",
			workspaceID,
		)
		breadcrumbs := []response.Breadcrumb{
			breadcrumb(
				"workspace-context",
				fmt.Sprintf("recuerd0 workspace context %d --pretty", workspaceID),
				"Load the workspace map and pinned context",
			),
			breadcrumb(
				"after-import-pass",
				afterImportPrompt,
				"Import leaves memories unstructured; cluster them, fix weak titles, and propose hubs for review",
			),
		}
		printImportCommitGuidance(summary, workspaceURL, breadcrumbs)
		printSuccessWithDetails(commitSummary, summary, workspaceURL, breadcrumbs)
	},
}

func importCommandError(err error) error {
	var cliError *errors.CLIError
	if stderrors.As(err, &cliError) {
		return cliError
	}
	var validation *importer.ValidationError
	if stderrors.As(err, &validation) {
		return errors.NewInvalidArgsError(validation.Error())
	}
	return errors.NewError(err.Error())
}

func displayPath(resolvedPath string) string {
	absolute, err := filepath.Abs(filepath.Clean(resolvedPath))
	if err != nil {
		return filepath.Clean(resolvedPath)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return absolute
	}
	relative, err := filepath.Rel(cwd, absolute)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return absolute
	}
	return filepath.Clean(relative)
}

func fetchImportWorkspace(apiClient client.API, workspaceID int64) (string, string, bool) {
	workspace, err := apiClient.Get(fmt.Sprintf("/workspaces/%d", workspaceID))
	if err != nil || workspace == nil {
		return "", "", false
	}
	data, ok := workspace.Data.(map[string]interface{})
	if !ok {
		return "", "", false
	}
	name, nameOK := data["name"].(string)
	url, urlOK := data["url"].(string)
	if !nameOK || strings.TrimSpace(name) == "" || !urlOK || strings.TrimSpace(url) == "" {
		return "", "", false
	}
	return name, url, true
}

func confirmImport(creates, versions int, workspaceID int64, workspaceName string) bool {
	fmt.Fprintf(
		importGuidanceOutput,
		"%s in %s? [y/N] ",
		importActions(creates, versions, "Create", "create"),
		importWorkspace(workspaceID, workspaceName),
	)
	reader := bufio.NewReader(importCommitInput)
	answer, err := reader.ReadString('\n')
	fmt.Fprintln(importGuidanceOutput)
	if err != nil && !stderrors.Is(err, io.EOF) {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(answer)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func importCommitSummary(creates, versions int, workspaceID int64, workspaceName string) string {
	return fmt.Sprintf(
		"%s in %s",
		importActions(creates, versions, "Created", "created"),
		importWorkspace(workspaceID, workspaceName),
	)
}

func importActions(creates, versions int, leading, fallback string) string {
	var actions []string
	if creates > 0 {
		actions = append(actions, fmt.Sprintf("%d %s", creates, pluralize(creates, "memory", "memories")))
	}
	if versions > 0 {
		actions = append(actions, fmt.Sprintf("%d %s", versions, pluralize(versions, "version", "versions")))
	}
	if len(actions) == 0 {
		return fallback
	}
	return leading + " " + strings.Join(actions, " and ")
}

func importWorkspace(workspaceID int64, workspaceName string) string {
	if workspaceName == "" {
		return fmt.Sprintf("workspace %d", workspaceID)
	}
	return fmt.Sprintf("workspace %d (%s)", workspaceID, workspaceName)
}

func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}

func printImportCommitGuidance(
	summary,
	workspaceURL string,
	breadcrumbs []response.Breadcrumb,
) {
	if !importGuidanceIsTTY() {
		return
	}
	fmt.Fprintln(importGuidanceOutput, summary)
	if workspaceURL != "" {
		fmt.Fprintf(importGuidanceOutput, "\nWorkspace: %s\n", workspaceURL)
	}
	fmt.Fprintln(importGuidanceOutput, "\nNext actions:")
	for _, next := range breadcrumbs {
		fmt.Fprintf(importGuidanceOutput, "  %s\n", next.Cmd)
	}
}

func printImportCommitFailureGuidance(ledgerPath, planPath string) {
	if !importGuidanceIsTTY() {
		return
	}
	fmt.Fprintf(importGuidanceOutput, importCommitFailureGuidance, ledgerPath, planPath)
}

func printImportGuidance(planPath string) {
	if !importGuidanceIsTTY() {
		return
	}
	fmt.Fprintf(importGuidanceOutput, importProposeGuidance, planPath, planPath)
}

func init() {
	rootCmd.AddCommand(importCmd)

	importProposeCmd.Flags().StringVar(&importProposeWorkspace, "workspace", "", "target workspace ID (required)")
	importProposeCmd.Flags().StringVar(&importProposePlan, "plan", "import.plan.yaml", "path for the reviewable plan")
	importProposeCmd.Flags().StringVar(&importProposeLedger, "ledger", "", "append-only ledger path (defaults beside the plan)")
	importProposeCmd.Flags().StringVar(&importProposeAdapter, "adapter", "", "source adapter: obsidian_markdown or workspace_export")
	importProposeCmd.Flags().BoolVar(&importProposeFresh, "fresh", false, "replace an existing plan instead of seeding from it")
	importCmd.AddCommand(importProposeCmd)

	importCommitCmd.Flags().BoolVar(&importCommitYes, "yes", false, "execute the reviewed plan")
	importCommitCmd.Flags().StringVar(&importCommitLedger, "ledger", "", "append-only ledger path (defaults beside the plan)")
	importCommitCmd.Flags().Lookup("yes").Usage = "skip the confirmation prompt; required when not interactive"
	importCmd.AddCommand(importCommitCmd)
}
