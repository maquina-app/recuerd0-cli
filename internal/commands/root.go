package commands

import (
	"fmt"
	"os"
	"sync"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/client"
	"github.com/maquina/recuerd0-cli/internal/config"
	"github.com/maquina/recuerd0-cli/internal/errors"
	"github.com/maquina/recuerd0-cli/internal/response"
)

var (
	// Global flag values
	cfgAccount   string
	cfgToken     string
	cfgAPIURL    string
	cfgWorkspace string
	cfgVerbose   bool
	cfgPretty    bool

	// Resolved configuration
	cfg *config.ResolvedConfig

	// Version (set by main.go)
	version = "dev"

	// Client factory (overridden in tests)
	clientFactory func() client.API
)

// SetVersion sets the version string displayed by the version command.
func SetVersion(v string) {
	version = v
}

var rootCmd = &cobra.Command{
	Use:   "recuerd0",
	Short: "Recuerd0 CLI — preserve, version, and organize knowledge from AI conversations",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if testMode {
			return
		}

		response.SetPrettyPrint(cfgPretty)

		// Skip config resolution for commands that don't need it
		if cmd.Name() == "version" {
			return
		}

		flags := config.ResolvedConfig{
			Account:   cfgAccount,
			Token:     cfgToken,
			APIURL:    cfgAPIURL,
			Workspace: cfgWorkspace,
		}
		resolved, err := config.Resolve(flags)
		if err != nil {
			exitWithError(errors.NewError(fmt.Sprintf("loading config: %v", err)))
			return
		}
		cfg = resolved
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgAccount, "account", "", "account name to use")
	rootCmd.PersistentFlags().StringVar(&cfgToken, "token", "", "API token (overrides config)")
	rootCmd.PersistentFlags().StringVar(&cfgAPIURL, "api-url", "", "API base URL (overrides config)")
	rootCmd.PersistentFlags().StringVar(&cfgWorkspace, "workspace", "", "workspace ID (overrides config)")
	rootCmd.PersistentFlags().BoolVar(&cfgVerbose, "verbose", false, "show HTTP request/response details")
	rootCmd.PersistentFlags().BoolVar(&cfgPretty, "pretty", false, "pretty-print JSON output")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// getClient creates an API client from the resolved config.
func getClient() client.API {
	if clientFactory != nil {
		return clientFactory()
	}
	return client.New(cfg.APIURL, cfg.Token, cfgVerbose)
}

// requireAuth checks that a token is available.
func requireAuth() error {
	if cfg == nil || cfg.Token == "" {
		return errors.NewAuthError("No API token configured. Run: recuerd0 account add <name> --token TOKEN")
	}
	return nil
}

// requireWorkspace checks that a workspace is available.
func requireWorkspace() (string, error) {
	if cfg == nil || cfg.Workspace == "" {
		return "", errors.NewInvalidArgsError("No workspace specified. Use --workspace flag or set workspace in .recuerd0.yaml")
	}
	return cfg.Workspace, nil
}

// exitWithError outputs an error response and exits.
func exitWithError(err error) {
	var cliErr *errors.CLIError
	switch e := err.(type) {
	case *errors.CLIError:
		cliErr = e
	default:
		cliErr = errors.NewError(err.Error())
	}

	resp := response.Error(cliErr)

	if testMode {
		testResult.Response = resp
		testResult.ExitCode = cliErr.ExitCode
		panic(testExitSignal{})
	}

	resp.PrintAndExit()
}

// printSuccess outputs a success response.
func printSuccess(data interface{}) {
	resp := response.Success(data)
	if testMode {
		testResult.Response = resp
		testResult.ExitCode = 0
		panic(testExitSignal{})
	}
	resp.Print()
}

// printSuccessWithLocation outputs a success response with location.
func printSuccessWithLocation(data interface{}, location string) {
	resp := response.SuccessWithLocation(data, location)
	if testMode {
		testResult.Response = resp
		testResult.ExitCode = 0
		panic(testExitSignal{})
	}
	resp.Print()
}

// printSuccessWithBreadcrumbs outputs a success response with summary and breadcrumbs.
func printSuccessWithBreadcrumbs(data interface{}, summary string, breadcrumbs []response.Breadcrumb) {
	resp := response.SuccessWithBreadcrumbs(data, summary, breadcrumbs)
	if testMode {
		testResult.Response = resp
		testResult.ExitCode = 0
		panic(testExitSignal{})
	}
	resp.Print()
}

// printSuccessWithPaginationAndBreadcrumbs outputs the full response.
func printSuccessWithPaginationAndBreadcrumbs(data interface{}, hasNext bool, nextURL string, summary string, breadcrumbs []response.Breadcrumb) {
	resp := response.SuccessWithPaginationAndBreadcrumbs(data, hasNext, nextURL, summary, breadcrumbs)
	if testMode {
		testResult.Response = resp
		testResult.ExitCode = 0
		panic(testExitSignal{})
	}
	resp.Print()
}

// breadcrumb is a helper to create a Breadcrumb.
func breadcrumb(action, cmd, description string) response.Breadcrumb {
	return response.Breadcrumb{Action: action, Cmd: cmd, Description: description}
}

// --- Test infrastructure ---

type testExitSignal struct{}

// CommandResult captures the result of a command in test mode.
type CommandResult struct {
	Response *response.Response
	ExitCode int
}

var (
	testMode   bool
	testResult *CommandResult
	testMu     sync.Mutex
)

// SetTestMode enables test mode with a mock client.
func SetTestMode(mockClient client.API) *CommandResult {
	testMu.Lock()
	testMode = true
	testResult = &CommandResult{}
	clientFactory = func() client.API { return mockClient }
	return testResult
}

// SetTestConfig sets the resolved config for tests.
func SetTestConfig(token, apiURL string) {
	cfg = &config.ResolvedConfig{
		Token:  token,
		APIURL: apiURL,
	}
}

// SetTestConfigFull sets the full resolved config for tests.
func SetTestConfigFull(token, apiURL, workspace string) {
	cfg = &config.ResolvedConfig{
		Token:     token,
		APIURL:    apiURL,
		Workspace: workspace,
	}
}

// ResetTestMode disables test mode and cleans up.
func ResetTestMode() {
	testMode = false
	testResult = nil
	clientFactory = nil
	cfg = nil
	testMu.Unlock()
}

// RunTestCommand runs a command function and captures the exit signal.
func RunTestCommand(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(testExitSignal); !ok {
				panic(r)
			}
		}
	}()
	fn()
}
