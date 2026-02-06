package commands

import "github.com/spf13/cobra"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the CLI version",
	Run: func(cmd *cobra.Command, args []string) {
		printSuccess(map[string]string{
			"version": version,
			"cli":     "recuerd0",
		})
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
