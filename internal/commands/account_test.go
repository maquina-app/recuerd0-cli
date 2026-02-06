package commands

import (
	"testing"

	"github.com/maquina/recuerd0-cli/internal/config"
)

func setupAccountTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	config.SetConfigDir(dir)
	t.Cleanup(func() { config.SetConfigDir("") })
}

func TestAccountAdd(t *testing.T) {
	setupAccountTest(t)

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	accountAddToken = "tok_test123"
	accountAddAPIURL = ""
	defer func() { accountAddToken = ""; accountAddAPIURL = "" }()

	RunTestCommand(func() {
		accountAddCmd.Run(accountAddCmd, []string{"personal"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if !result.Response.Success {
		t.Error("expected success response")
	}

	// Verify account was saved
	globalCfg, _ := config.LoadGlobal()
	if _, ok := globalCfg.Accounts["personal"]; !ok {
		t.Error("expected 'personal' account to exist")
	}
	if globalCfg.Current != "personal" {
		t.Errorf("expected current 'personal', got %q", globalCfg.Current)
	}
}

func TestAccountAdd_MissingToken(t *testing.T) {
	setupAccountTest(t)

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	accountAddToken = ""
	defer func() { accountAddToken = "" }()

	RunTestCommand(func() {
		accountAddCmd.Run(accountAddCmd, []string{"personal"})
	})

	if result.Response.Success {
		t.Error("expected error response")
	}
	if result.ExitCode == 0 {
		t.Error("expected non-zero exit code")
	}
}

func TestAccountList(t *testing.T) {
	setupAccountTest(t)

	// Add accounts first
	_ = config.AddAccount("personal", "tok_a", "")
	_ = config.AddAccount("work", "tok_b", "https://work.example.com")

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	RunTestCommand(func() {
		accountListCmd.Run(accountListCmd, []string{})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if !result.Response.Success {
		t.Error("expected success response")
	}
	if result.Response.Summary != "2 account(s)" {
		t.Errorf("expected summary '2 account(s)', got %q", result.Response.Summary)
	}
}

func TestAccountSelect(t *testing.T) {
	setupAccountTest(t)

	_ = config.AddAccount("a", "tok_a", "")
	_ = config.AddAccount("b", "tok_b", "")

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	RunTestCommand(func() {
		accountSelectCmd.Run(accountSelectCmd, []string{"b"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	globalCfg, _ := config.LoadGlobal()
	if globalCfg.Current != "b" {
		t.Errorf("expected current 'b', got %q", globalCfg.Current)
	}
}

func TestAccountSelect_NotFound(t *testing.T) {
	setupAccountTest(t)

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	RunTestCommand(func() {
		accountSelectCmd.Run(accountSelectCmd, []string{"nonexistent"})
	})

	if result.Response.Success {
		t.Error("expected error response")
	}
}

func TestAccountRemove(t *testing.T) {
	setupAccountTest(t)

	_ = config.AddAccount("only", "tok_only", "")

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	RunTestCommand(func() {
		accountRemoveCmd.Run(accountRemoveCmd, []string{"only"})
	})

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}

	globalCfg, _ := config.LoadGlobal()
	if len(globalCfg.Accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(globalCfg.Accounts))
	}
}

func TestAccountRemove_RefuseCurrentWithOthers(t *testing.T) {
	setupAccountTest(t)

	_ = config.AddAccount("a", "tok_a", "")
	_ = config.AddAccount("b", "tok_b", "")

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	RunTestCommand(func() {
		accountRemoveCmd.Run(accountRemoveCmd, []string{"a"})
	})

	if result.Response.Success {
		t.Error("expected error when removing current account with others present")
	}
}
