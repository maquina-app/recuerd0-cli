package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	SetConfigDir(dir)
	t.Cleanup(func() { SetConfigDir("") })
	return dir
}

func TestLoadGlobal_NoFile(t *testing.T) {
	setupTestDir(t)

	cfg, err := LoadGlobal()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Current != "" {
		t.Errorf("expected empty current, got %q", cfg.Current)
	}
	if len(cfg.Accounts) != 0 {
		t.Errorf("expected no accounts, got %d", len(cfg.Accounts))
	}
}

func TestSaveAndLoadGlobal(t *testing.T) {
	setupTestDir(t)

	cfg := &GlobalConfig{
		Current: "personal",
		Accounts: map[string]AccountConfig{
			"personal": {Token: "tok_abc", APIURL: "https://recuerd0.ai"},
		},
	}
	if err := SaveGlobal(cfg); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := LoadGlobal()
	if err != nil {
		t.Fatalf("load error: %v", err)
	}
	if loaded.Current != "personal" {
		t.Errorf("expected current 'personal', got %q", loaded.Current)
	}
	acct, ok := loaded.Accounts["personal"]
	if !ok {
		t.Fatal("expected 'personal' account")
	}
	if acct.Token != "tok_abc" {
		t.Errorf("expected token 'tok_abc', got %q", acct.Token)
	}
}

func TestAddAccount_FirstBecomesDefault(t *testing.T) {
	setupTestDir(t)

	if err := AddAccount("work", "tok_123", "https://work.recuerd0.ai"); err != nil {
		t.Fatalf("add error: %v", err)
	}

	cfg, _ := LoadGlobal()
	if cfg.Current != "work" {
		t.Errorf("expected current 'work', got %q", cfg.Current)
	}
	if cfg.Accounts["work"].APIURL != "https://work.recuerd0.ai" {
		t.Errorf("unexpected api_url: %s", cfg.Accounts["work"].APIURL)
	}
}

func TestAddAccount_DefaultAPIURL(t *testing.T) {
	setupTestDir(t)

	if err := AddAccount("personal", "tok_abc", ""); err != nil {
		t.Fatalf("add error: %v", err)
	}

	cfg, _ := LoadGlobal()
	if cfg.Accounts["personal"].APIURL != DefaultAPIURL {
		t.Errorf("expected default API URL, got %q", cfg.Accounts["personal"].APIURL)
	}
}

func TestAddAccount_SecondDoesNotChangeCurrent(t *testing.T) {
	setupTestDir(t)

	_ = AddAccount("first", "tok_1", "")
	_ = AddAccount("second", "tok_2", "")

	cfg, _ := LoadGlobal()
	if cfg.Current != "first" {
		t.Errorf("expected current 'first', got %q", cfg.Current)
	}
}

func TestSetCurrent(t *testing.T) {
	setupTestDir(t)

	_ = AddAccount("a", "tok_a", "")
	_ = AddAccount("b", "tok_b", "")

	if err := SetCurrent("b"); err != nil {
		t.Fatalf("set current error: %v", err)
	}

	cfg, _ := LoadGlobal()
	if cfg.Current != "b" {
		t.Errorf("expected current 'b', got %q", cfg.Current)
	}
}

func TestSetCurrent_NotFound(t *testing.T) {
	setupTestDir(t)

	err := SetCurrent("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent account")
	}
}

func TestRemoveAccount(t *testing.T) {
	setupTestDir(t)

	_ = AddAccount("only", "tok_only", "")
	if err := RemoveAccount("only"); err != nil {
		t.Fatalf("remove error: %v", err)
	}

	cfg, _ := LoadGlobal()
	if len(cfg.Accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(cfg.Accounts))
	}
	if cfg.Current != "" {
		t.Errorf("expected empty current, got %q", cfg.Current)
	}
}

func TestRemoveAccount_RefuseCurrentWithOthers(t *testing.T) {
	setupTestDir(t)

	_ = AddAccount("a", "tok_a", "")
	_ = AddAccount("b", "tok_b", "")

	err := RemoveAccount("a")
	if err == nil {
		t.Error("expected error when removing current account with others present")
	}
}

func TestRemoveAccount_NotFound(t *testing.T) {
	setupTestDir(t)

	err := RemoveAccount("ghost")
	if err == nil {
		t.Error("expected error for nonexistent account")
	}
}

func TestFindLocal(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "a", "b")
	os.MkdirAll(sub, 0755)

	content := []byte("account: work\nworkspace: \"5\"\n")
	os.WriteFile(filepath.Join(dir, localFileName), content, 0644)

	cfg, err := FindLocal(sub)
	if err != nil {
		t.Fatalf("find local error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected local config to be found")
	}
	if cfg.Account != "work" {
		t.Errorf("expected account 'work', got %q", cfg.Account)
	}
	if cfg.Workspace != "5" {
		t.Errorf("expected workspace '5', got %q", cfg.Workspace)
	}
}

func TestFindLocal_NotFound(t *testing.T) {
	dir := t.TempDir()
	cfg, err := FindLocal(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil when no local config exists")
	}
}

func TestResolve_FlagOverrides(t *testing.T) {
	setupTestDir(t)
	_ = AddAccount("default", "tok_default", "https://default.api")

	// Clear env vars
	os.Unsetenv("RECUERD0_ACCOUNT")
	os.Unsetenv("RECUERD0_TOKEN")
	os.Unsetenv("RECUERD0_API_URL")
	os.Unsetenv("RECUERD0_WORKSPACE")

	flags := ResolvedConfig{
		Token:     "tok_flag",
		APIURL:    "https://flag.api",
		Account:   "default",
		Workspace: "42",
	}
	resolved, err := Resolve(flags)
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if resolved.Token != "tok_flag" {
		t.Errorf("expected token 'tok_flag', got %q", resolved.Token)
	}
	if resolved.APIURL != "https://flag.api" {
		t.Errorf("expected api url 'https://flag.api', got %q", resolved.APIURL)
	}
	if resolved.Workspace != "42" {
		t.Errorf("expected workspace '42', got %q", resolved.Workspace)
	}
}

func TestResolve_DefaultAPIURL(t *testing.T) {
	setupTestDir(t)

	os.Unsetenv("RECUERD0_ACCOUNT")
	os.Unsetenv("RECUERD0_TOKEN")
	os.Unsetenv("RECUERD0_API_URL")
	os.Unsetenv("RECUERD0_WORKSPACE")

	resolved, err := Resolve(ResolvedConfig{})
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if resolved.APIURL != DefaultAPIURL {
		t.Errorf("expected default API URL %q, got %q", DefaultAPIURL, resolved.APIURL)
	}
}
