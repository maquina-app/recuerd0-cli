package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"gopkg.in/yaml.v3"
)

const (
	DefaultAPIURL  = "https://recuerd0.ai"
	globalDir      = "recuerd0"
	globalFileName = "config.yaml"
	localFileName  = ".recuerd0.yaml"
)

// AccountConfig holds credentials for a single named account.
type AccountConfig struct {
	Token  string `yaml:"token"`
	APIURL string `yaml:"api_url"`
}

// GlobalConfig is the top-level config stored at ~/.config/recuerd0/config.yaml.
type GlobalConfig struct {
	Current  string                   `yaml:"current"`
	Accounts map[string]AccountConfig `yaml:"accounts"`
}

// LocalConfig is an optional per-project override at .recuerd0.yaml.
type LocalConfig struct {
	Account   string `yaml:"account"`
	Workspace string `yaml:"workspace"`
}

// ResolvedConfig is the final merged configuration used by commands.
type ResolvedConfig struct {
	Token     string
	APIURL    string
	Account   string
	Workspace string
}

// globalConfigPath returns the path to the global config file.
// It can be overridden via configDir for testing.
var configDir string

func SetConfigDir(dir string) {
	configDir = dir
}

func globalConfigPath() string {
	if configDir != "" {
		return filepath.Join(configDir, globalFileName)
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, globalDir, globalFileName)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.Getenv("HOME")
	}
	return filepath.Join(home, ".config", globalDir, globalFileName)
}

// DEPRECATED: Remove this migration function in the next release.
// migrateFromLegacyPath moves the config from the old macOS-specific path
// (~/Library/Application Support/recuerd0/config.yaml) to the new XDG path
// (~/.config/recuerd0/config.yaml).
func migrateFromLegacyPath() {
	if runtime.GOOS != "darwin" {
		return
	}
	if configDir != "" {
		return
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	oldPath := filepath.Join(home, "Library", "Application Support", globalDir, globalFileName)

	// Use the same logic as globalConfigPath to determine the new path
	newBase := os.Getenv("XDG_CONFIG_HOME")
	if newBase == "" {
		newBase = filepath.Join(home, ".config")
	}
	newPath := filepath.Join(newBase, globalDir, globalFileName)

	if _, err := os.Stat(newPath); err == nil {
		return
	}
	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return
	}

	if err := os.MkdirAll(filepath.Dir(newPath), 0700); err != nil {
		return
	}
	os.Rename(oldPath, newPath)
}

// LoadGlobal reads the global config from disk.
func LoadGlobal() (*GlobalConfig, error) {
	// DEPRECATED: Remove this call in the next release.
	migrateFromLegacyPath()
	path := globalConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &GlobalConfig{Accounts: make(map[string]AccountConfig)}, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg GlobalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	if cfg.Accounts == nil {
		cfg.Accounts = make(map[string]AccountConfig)
	}
	return &cfg, nil
}

// SaveGlobal writes the global config to disk.
func SaveGlobal(cfg *GlobalConfig) error {
	path := globalConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// FindLocal walks up from startDir looking for .recuerd0.yaml.
func FindLocal(startDir string) (*LocalConfig, error) {
	dir := startDir
	for {
		path := filepath.Join(dir, localFileName)
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg LocalConfig
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return nil, fmt.Errorf("parsing local config: %w", err)
			}
			return &cfg, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return nil, nil
}

// Resolve merges all config layers into a ResolvedConfig.
// Priority: flags > env > local config > global config.
func Resolve(flags ResolvedConfig) (*ResolvedConfig, error) {
	global, err := LoadGlobal()
	if err != nil {
		return nil, err
	}

	cwd, _ := os.Getwd()
	local, _ := FindLocal(cwd)

	resolved := &ResolvedConfig{}

	// Start with global config (current account)
	accountName := global.Current
	if local != nil && local.Account != "" {
		accountName = local.Account
	}
	if env := os.Getenv("RECUERD0_ACCOUNT"); env != "" {
		accountName = env
	}
	if flags.Account != "" {
		accountName = flags.Account
	}
	resolved.Account = accountName

	if acct, ok := global.Accounts[accountName]; ok {
		resolved.Token = acct.Token
		resolved.APIURL = acct.APIURL
	}

	// Workspace from local config
	if local != nil && local.Workspace != "" {
		resolved.Workspace = local.Workspace
	}

	// Env overrides
	if env := os.Getenv("RECUERD0_TOKEN"); env != "" {
		resolved.Token = env
	}
	if env := os.Getenv("RECUERD0_API_URL"); env != "" {
		resolved.APIURL = env
	}
	if env := os.Getenv("RECUERD0_WORKSPACE"); env != "" {
		resolved.Workspace = env
	}

	// Flag overrides
	if flags.Token != "" {
		resolved.Token = flags.Token
	}
	if flags.APIURL != "" {
		resolved.APIURL = flags.APIURL
	}
	if flags.Workspace != "" {
		resolved.Workspace = flags.Workspace
	}

	// Default API URL
	if resolved.APIURL == "" {
		resolved.APIURL = DefaultAPIURL
	}

	return resolved, nil
}

// AddAccount adds or updates a named account in the global config.
// If it's the first account, it becomes the current account.
func AddAccount(name, token, apiURL string) error {
	cfg, err := LoadGlobal()
	if err != nil {
		return err
	}
	if apiURL == "" {
		apiURL = DefaultAPIURL
	}
	cfg.Accounts[name] = AccountConfig{Token: token, APIURL: apiURL}
	if cfg.Current == "" || len(cfg.Accounts) == 1 {
		cfg.Current = name
	}
	return SaveGlobal(cfg)
}

// RemoveAccount removes a named account from the global config.
func RemoveAccount(name string) error {
	cfg, err := LoadGlobal()
	if err != nil {
		return err
	}
	if _, ok := cfg.Accounts[name]; !ok {
		return fmt.Errorf("account %q not found", name)
	}
	if cfg.Current == name && len(cfg.Accounts) > 1 {
		return fmt.Errorf("cannot remove current account %q while other accounts exist; switch to another account first", name)
	}
	delete(cfg.Accounts, name)
	if cfg.Current == name {
		cfg.Current = ""
	}
	return SaveGlobal(cfg)
}

// SetCurrent sets the active account.
func SetCurrent(name string) error {
	cfg, err := LoadGlobal()
	if err != nil {
		return err
	}
	if _, ok := cfg.Accounts[name]; !ok {
		return fmt.Errorf("account %q not found", name)
	}
	cfg.Current = name
	return SaveGlobal(cfg)
}

// ListAccounts returns all accounts and which is current.
func ListAccounts() (*GlobalConfig, error) {
	return LoadGlobal()
}
