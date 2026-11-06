package commands

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/maquina/recuerd0-cli/internal/client"
	clierrors "github.com/maquina/recuerd0-cli/internal/errors"
)

func commandTree() *cobra.Command {
	root := &cobra.Command{Use: "root", SilenceUsage: true, SilenceErrors: true}
	group := &cobra.Command{Use: "group", Short: "A group"}
	group.AddCommand(&cobra.Command{Use: "leaf", Run: func(*cobra.Command, []string) {}})
	root.AddCommand(group)
	rejectUnknownSubcommands(root)
	return root
}

func TestUnknownSubcommandIsAnError(t *testing.T) {
	root := commandTree()
	root.SetArgs([]string{"group", "nope"})
	if _, err := root.ExecuteC(); err == nil || !strings.Contains(err.Error(), `unknown command "nope"`) {
		t.Fatalf("expected an error naming it, got %v", err)
	}
}

func TestGroupAloneStillShowsHelp(t *testing.T) {
	root := commandTree()
	root.SetArgs([]string{"group"})
	var help strings.Builder
	root.SetOut(&help)
	if _, err := root.ExecuteC(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(help.String(), "leaf") {
		t.Fatalf("expected help, got %q", help.String())
	}
}

func TestUsageErrorIsAnInvalidArgsErrorWithTheUsage(t *testing.T) {
	root := commandTree()
	root.SetArgs([]string{"group", "leaf", "--nope"})
	cmd, err := root.ExecuteC()
	if err == nil {
		t.Fatal("expected an unknown flag error")
	}
	cliErr, ok := usageError(cmd, err).(*clierrors.CLIError)
	if !ok || cliErr.ExitCode != clierrors.ExitInvalidArgs || !strings.Contains(cliErr.Message, "usage: root group leaf") {
		t.Fatalf("got %#v", cliErr)
	}
}

func TestMemoryCreateWithNothingSaysSoWithoutCallingTheServer(t *testing.T) {
	mock := NewMockClient()
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "1")
	defer ResetTestMode()

	RunTestCommand(func() { memoryCreateCmd.Run(memoryCreateCmd, nil) })

	if result.ExitCode != clierrors.ExitInvalidArgs || len(mock.PostCalls) != 0 {
		t.Fatalf("expected an argument error and no request, got exit %d and %d calls", result.ExitCode, len(mock.PostCalls))
	}
}

func TestMemoryUpdateEmptyTagsClearsThem(t *testing.T) {
	mock := NewMockClient()
	mock.PatchResponse = &client.APIResponse{StatusCode: 200, Data: map[string]interface{}{"id": 1}}
	result := SetTestMode(mock)
	SetTestConfigFull("tok", "https://api.example.com", "1")
	defer ResetTestMode()

	if err := memoryUpdateCmd.Flags().Set("tags", ""); err != nil {
		t.Fatal(err)
	}
	defer func() {
		memoryUpdateTags = ""
		memoryUpdateCmd.Flags().Lookup("tags").Changed = false
	}()

	RunTestCommand(func() { memoryUpdateCmd.Run(memoryUpdateCmd, []string{"1"}) })

	if result.ExitCode != 0 || len(mock.PatchCalls) != 1 {
		t.Fatalf("expected one request, got exit %d and %d calls", result.ExitCode, len(mock.PatchCalls))
	}
	memory := mock.PatchCalls[0].Body.(map[string]interface{})["memory"].(map[string]interface{})
	if tags, ok := memory["tags"].([]string); !ok || len(tags) != 0 {
		t.Fatalf("expected empty tags to be sent, got %#v", memory["tags"])
	}
}
