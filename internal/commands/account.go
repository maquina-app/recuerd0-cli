package commands

import (
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/client"
	"github.com/maquina/recuerd0-cli/internal/config"
	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/response"
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Manage configured accounts",
}

// account add
var (
	accountAddToken      string
	accountAddAPIURL     string
	accountAddSkipVerify bool
)

var accountAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if accountAddToken == "" {
			exitWithError(errors.NewInvalidArgsError("--token is required"))
			return
		}

		apiURL := accountAddAPIURL
		if apiURL == "" {
			apiURL = config.DefaultAPIURL
		}

		summary := fmt.Sprintf("Account %q added and verified", name)
		if accountAddSkipVerify {
			summary = fmt.Sprintf("Account %q added without verification", name)
		} else {
			candidateClient := client.New(apiURL, accountAddToken, cfgVerbose)
			if _, err := candidateClient.GetWithPagination("/workspaces"); err != nil {
				exitWithError(accountVerificationError(apiURL, accountAddToken, err))
				return
			}
		}

		if err := config.AddAccount(name, accountAddToken, apiURL); err != nil {
			exitWithError(errors.NewError(fmt.Sprintf("adding account: %v", err)))
			return
		}

		data := map[string]string{
			"name":    name,
			"api_url": apiURL,
		}

		printSuccessWithBreadcrumbs(data, summary, []response.Breadcrumb{
			breadcrumb("list", "recuerd0 account list", "List all accounts"),
			breadcrumb("select", fmt.Sprintf("recuerd0 account select %s", name), "Switch to this account"),
		})
	},
}

// account list
var accountListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured accounts",
	Run: func(cmd *cobra.Command, args []string) {
		globalCfg, err := config.ListAccounts()
		if err != nil {
			exitWithError(errors.NewError(fmt.Sprintf("listing accounts: %v", err)))
			return
		}

		type accountEntry struct {
			Name    string `json:"name"`
			APIURL  string `json:"api_url"`
			Current bool   `json:"current"`
		}

		accounts := make([]accountEntry, 0, len(globalCfg.Accounts))
		for name, acct := range globalCfg.Accounts {
			accounts = append(accounts, accountEntry{
				Name:    name,
				APIURL:  acct.APIURL,
				Current: name == globalCfg.Current,
			})
		}

		summary := fmt.Sprintf("%d account(s)", len(accounts))
		bc := []response.Breadcrumb{
			breadcrumb("add", "recuerd0 account add <name> --token TOKEN", "Add a new account"),
		}

		printSuccessWithBreadcrumbs(accounts, summary, bc)
	},
}

// account select
var accountSelectCmd = &cobra.Command{
	Use:   "select <name>",
	Short: "Set the active account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if err := config.SetCurrent(name); err != nil {
			exitWithError(errors.NewError(fmt.Sprintf("selecting account: %v", err)))
			return
		}

		printSuccessWithBreadcrumbs(
			map[string]string{"current": name},
			fmt.Sprintf("Switched to account %q", name),
			[]response.Breadcrumb{
				breadcrumb("list", "recuerd0 account list", "List all accounts"),
				breadcrumb("workspaces", "recuerd0 workspace list", "List workspaces"),
			},
		)
	},
}

// account remove
var accountRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove an account",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		if err := config.RemoveAccount(name); err != nil {
			exitWithError(errors.NewError(fmt.Sprintf("removing account: %v", err)))
			return
		}

		printSuccessWithBreadcrumbs(
			map[string]string{"removed": name},
			fmt.Sprintf("Account %q removed", name),
			[]response.Breadcrumb{
				breadcrumb("list", "recuerd0 account list", "List remaining accounts"),
			},
		)
	},
}

func accountVerificationError(apiURL, token string, err error) error {
	var cliErr *errors.CLIError
	if !stderrors.As(err, &cliErr) {
		message := redactToken(fmt.Sprintf("verifying account against API URL %q: %v", apiURL, err), token)
		return errors.NewError(message)
	}

	switch cliErr.Code {
	case errors.CodeAuth:
		return errors.NewAuthError(redactToken(
			fmt.Sprintf("Account verification failed: the token was rejected by API URL %q", apiURL),
			token,
		))
	case errors.CodeNetwork:
		return errors.NewNetworkError(redactToken(
			fmt.Sprintf("Account verification failed: API URL %q is unreachable: %s", apiURL, cliErr.Message),
			token,
		))
	default:
		return &errors.CLIError{
			Code: cliErr.Code,
			Message: redactToken(
				fmt.Sprintf("Account verification failed against API URL %q: %s", apiURL, cliErr.Message),
				token,
			),
			Status:   cliErr.Status,
			ExitCode: cliErr.ExitCode,
		}
	}
}

func redactToken(message, token string) string {
	if token == "" {
		return message
	}
	return strings.ReplaceAll(message, token, "[REDACTED]")
}

func init() {
	rootCmd.AddCommand(accountCmd)

	accountAddCmd.Flags().StringVar(&accountAddToken, "token", "", "API token (required)")
	accountAddCmd.Flags().StringVar(&accountAddAPIURL, "api-url", "", "API base URL (default: https://recuerd0.ai)")
	accountAddCmd.Flags().BoolVar(&accountAddSkipVerify, "skip-verify", false, "save credentials without contacting the API")
	accountCmd.AddCommand(accountAddCmd)

	accountCmd.AddCommand(accountListCmd)
	accountCmd.AddCommand(accountSelectCmd)
	accountCmd.AddCommand(accountRemoveCmd)
}
