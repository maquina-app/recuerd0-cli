package commands

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/maquina/recuerd0-cli/internal/config"
	"github.com/maquina/recuerd0-cli/internal/errors"
)

func setupAccountTest(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	config.SetConfigDir(dir)
	t.Cleanup(func() {
		config.SetConfigDir("")
		accountAddToken = ""
		accountAddAPIURL = ""
		accountAddSkipVerify = false
	})
}

func TestAccountAdd_VerifiesSubmittedCredentialsBeforeSaving(t *testing.T) {
	setupAccountTest(t)

	const token = "tok_test123"
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/workspaces" {
			t.Errorf("expected /workspaces, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+token {
			t.Errorf("unexpected Authorization header %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, "[]")
	}))
	defer server.Close()

	result := runAccountAddTest("personal", token, server.URL, false)

	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
	if !result.Response.Success {
		t.Error("expected success response")
	}
	if result.Response.Summary != `Account "personal" added and verified` {
		t.Errorf("unexpected summary %q", result.Response.Summary)
	}
	if requests.Load() != 1 {
		t.Errorf("expected one verification request, got %d", requests.Load())
	}

	// Verify account was saved
	globalCfg, _ := config.LoadGlobal()
	account, ok := globalCfg.Accounts["personal"]
	if !ok {
		t.Error("expected 'personal' account to exist")
	}
	if account.Token != token || account.APIURL != server.URL {
		t.Errorf("submitted credentials were not saved: %#v", account)
	}
	if globalCfg.Current != "personal" {
		t.Errorf("expected current 'personal', got %q", globalCfg.Current)
	}
}

func TestAccountAdd_RejectedTokenDoesNotPersist(t *testing.T) {
	setupAccountTest(t)

	const token = "tok_rejected"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error":"rejected %s"}`, token)
	}))
	defer server.Close()

	result := runAccountAddTest("personal", token, server.URL, false)
	if result.ExitCode != errors.ExitAuthFailure {
		t.Fatalf("expected auth exit %d, got %d", errors.ExitAuthFailure, result.ExitCode)
	}
	if result.Response.Error == nil || result.Response.Error.Code != errors.CodeAuth {
		t.Fatalf("expected auth error, got %#v", result.Response)
	}
	if !strings.Contains(result.Response.Error.Message, "token was rejected") {
		t.Errorf("expected rejection context, got %q", result.Response.Error.Message)
	}
	if strings.Contains(result.Response.Error.Message, token) {
		t.Errorf("error leaked token: %q", result.Response.Error.Message)
	}
	globalCfg, _ := config.LoadGlobal()
	if _, ok := globalCfg.Accounts["personal"]; ok {
		t.Error("rejected credentials must not be persisted")
	}
}

func TestAccountAdd_UnreachableAPIDoesNotPersist(t *testing.T) {
	setupAccountTest(t)

	const token = "tok_network"
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	apiURL := server.URL
	server.Close()

	result := runAccountAddTest("personal", token, apiURL, false)
	if result.ExitCode != errors.ExitNetwork {
		t.Fatalf("expected network exit %d, got %d", errors.ExitNetwork, result.ExitCode)
	}
	if result.Response.Error == nil || result.Response.Error.Code != errors.CodeNetwork {
		t.Fatalf("expected network error, got %#v", result.Response)
	}
	if !strings.Contains(result.Response.Error.Message, apiURL) || !strings.Contains(result.Response.Error.Message, "unreachable") {
		t.Errorf("expected unreachable API URL context, got %q", result.Response.Error.Message)
	}
	if strings.Contains(result.Response.Error.Message, token) {
		t.Errorf("error leaked token: %q", result.Response.Error.Message)
	}
	globalCfg, _ := config.LoadGlobal()
	if _, ok := globalCfg.Accounts["personal"]; ok {
		t.Error("unreachable credentials must not be persisted")
	}
}

func TestAccountAdd_FailedReplacementPreservesExistingAccount(t *testing.T) {
	setupAccountTest(t)

	if err := config.AddAccount("personal", "tok_original", "https://original.example.com"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	result := runAccountAddTest("personal", "tok_replacement", server.URL, false)
	if result.ExitCode != errors.ExitAuthFailure {
		t.Fatalf("expected auth failure, got %#v", result)
	}

	globalCfg, _ := config.LoadGlobal()
	account := globalCfg.Accounts["personal"]
	if account.Token != "tok_original" || account.APIURL != "https://original.example.com" {
		t.Errorf("failed verification changed existing account: %#v", account)
	}
}

func TestAccountAdd_SkipVerifyPersistsWithoutRequest(t *testing.T) {
	setupAccountTest(t)

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	result := runAccountAddTest("offline", "tok_offline", server.URL, true)
	if result.ExitCode != 0 || result.Response == nil || !result.Response.Success {
		t.Fatalf("skip verify failed: %#v", result)
	}
	if result.Response.Summary != `Account "offline" added without verification` {
		t.Errorf("unexpected summary %q", result.Response.Summary)
	}
	if requests.Load() != 0 {
		t.Errorf("--skip-verify made %d request(s)", requests.Load())
	}
	globalCfg, _ := config.LoadGlobal()
	account, ok := globalCfg.Accounts["offline"]
	if !ok || account.Token != "tok_offline" || account.APIURL != server.URL {
		t.Errorf("skipped credentials were not saved: %#v", account)
	}
}

func TestAccountAdd_SkipVerifyUsesDefaultAPIURL(t *testing.T) {
	setupAccountTest(t)

	flag := accountAddCmd.Flags().Lookup("skip-verify")
	if flag == nil || flag.DefValue != "false" {
		t.Fatalf("--skip-verify must exist and default to false: %#v", flag)
	}

	result := runAccountAddTest("offline", "tok_offline", "", true)
	if result.ExitCode != 0 || result.Response == nil || !result.Response.Success {
		t.Fatalf("skip verify failed: %#v", result)
	}
	globalCfg, _ := config.LoadGlobal()
	if got := globalCfg.Accounts["offline"].APIURL; got != config.DefaultAPIURL {
		t.Errorf("expected default API URL %q, got %q", config.DefaultAPIURL, got)
	}
	data, ok := result.Response.Data.(map[string]string)
	if !ok || data["api_url"] != config.DefaultAPIURL {
		t.Errorf("success data did not retain the default API URL: %#v", result.Response.Data)
	}
}

func TestAccountAdd_OtherHTTPErrorKeepsTypeAndAddsContext(t *testing.T) {
	setupAccountTest(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, `{"error":"not allowed"}`)
	}))
	defer server.Close()

	result := runAccountAddTest("personal", "tok_forbidden", server.URL, false)
	if result.ExitCode != errors.ExitForbidden {
		t.Fatalf("expected forbidden exit %d, got %d", errors.ExitForbidden, result.ExitCode)
	}
	if result.Response.Error == nil ||
		result.Response.Error.Code != errors.CodeForbidden ||
		result.Response.Error.Status != http.StatusForbidden {
		t.Fatalf("verification changed typed HTTP error: %#v", result.Response)
	}
	if !strings.Contains(result.Response.Error.Message, "Account verification failed") {
		t.Errorf("missing verification context: %q", result.Response.Error.Message)
	}
}

func runAccountAddTest(name, token, apiURL string, skipVerify bool) *CommandResult {
	result := SetTestMode(NewMockClient())
	defer ResetTestMode()

	accountAddToken = token
	accountAddAPIURL = apiURL
	accountAddSkipVerify = skipVerify
	defer func() {
		accountAddToken = ""
		accountAddAPIURL = ""
		accountAddSkipVerify = false
	}()

	RunTestCommand(func() {
		accountAddCmd.Run(accountAddCmd, []string{name})
	})
	return result
}

func TestAccountAdd_MissingToken(t *testing.T) {
	setupAccountTest(t)

	mock := NewMockClient()
	result := SetTestMode(mock)
	defer ResetTestMode()

	accountAddToken = ""

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
