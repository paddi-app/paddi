package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// var (not const) so it can be overridden at build time via -ldflags -X.
var DefaultAPIBase = "http://localhost:8601"

// Environment variables and config file keys, centralized so callers never
// spell them out as string literals.
const (
	EnvConfigPath    = "PADDI_CONFIG"
	EnvXDGConfigHome = "XDG_CONFIG_HOME"
	EnvAPIBase       = "PADDI_API_BASE"
	EnvProject       = "PADDI_PROJECT"
	EnvToken         = "PADDI_TOKEN"

	KeyAPIBase     = "api_base"
	KeyWorkspaceID = "context.workspace_id"
	KeyProjectID   = "context.project_id"
)

// fileConfig mirrors the on-disk TOML config file shape.
type fileConfig struct {
	APIBase string `toml:"api_base,omitempty"`
	Context struct {
		WorkspaceID string `toml:"workspace_id,omitempty"`
		ProjectID   string `toml:"project_id,omitempty"`
	} `toml:"context"`
}

// Path returns the config file location, honoring PADDI_CONFIG.
func Path() (string, error) {
	if p := os.Getenv(EnvConfigPath); p != "" {
		return p, nil
	}
	dir := os.Getenv(EnvXDGConfigHome)
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "paddi", "config.toml"), nil
}

// Clear removes the config file. A missing file is not an error.
func Clear() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Set updates a single dotted key (KeyWorkspaceID or KeyProjectID) in the
// config file, creating the file if needed.
func Set(key, value string) error {
	path, err := Path()
	if err != nil {
		return err
	}
	var cfg fileConfig
	if data, err := os.ReadFile(path); err == nil {
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("load config %s: %w", path, err)
		}
	}
	switch key {
	case KeyWorkspaceID:
		cfg.Context.WorkspaceID = value
	case KeyProjectID:
		cfg.Context.ProjectID = value
	default:
		return fmt.Errorf("unknown config key %q", key)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}
