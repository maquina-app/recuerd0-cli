package commands

import (
	stderrors "errors"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/importer"
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
		_, digest, err := importer.Propose(getClient(), importer.ProposeOptions{
			SourcePath: args[0], PlanPath: importProposePlan,
			LedgerPath: importProposeLedger, Adapter: importProposeAdapter,
			Workspace: workspace, Fresh: importProposeFresh,
		})
		if err != nil {
			exitWithError(importCommandError(err))
			return
		}
		printSuccess(digest)
	},
}

var (
	importCommitYes    bool
	importCommitLedger string
	importCommitDryRun bool
)

var importCommitCmd = &cobra.Command{
	Use:   "commit <plan>",
	Short: "Validate and execute an approved import plan",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		options := importer.CommitOptions{PlanPath: args[0], LedgerPath: importCommitLedger}
		if importCommitDryRun || !importCommitYes {
			_, digest, _, err := importer.Review(options)
			if err != nil {
				exitWithError(importCommandError(err))
				return
			}
			printSuccessWithExitCode(digest, 1)
			return
		}
		if err := requireAuth(); err != nil {
			exitWithError(err)
			return
		}
		summary, err := importer.Commit(getClient(), options)
		if err != nil {
			printSuccessWithExitCode(summary, 2)
			return
		}
		printSuccess(summary)
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
	importCommitCmd.Flags().BoolVar(&importCommitDryRun, "dry-run", false, "validate and print the digest without writes")
	importCmd.AddCommand(importCommitCmd)
}
