package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
)

func tempConfigPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv(EnvConfigPath, path)
	return path
}

func TestPath_HonorsEnvOverride(t *testing.T) {
	want := tempConfigPath(t)
	got, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestPath_DefaultsUnderXDGConfigHome(t *testing.T) {
	t.Setenv(EnvConfigPath, "")
	dir := t.TempDir()
	t.Setenv(EnvXDGConfigHome, dir)

	got, err := Path()
	if err != nil {
		t.Fatalf("Path() error = %v", err)
	}
	want := filepath.Join(dir, "paddi", "config.toml")
	if got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

func TestSet_WritesAndOverwritesKey(t *testing.T) {
	path := tempConfigPath(t)

	if err := Set(KeyProjectID, "p1"); err != nil {
		t.Fatalf("Set(project) error = %v", err)
	}
	if err := Set(KeyWorkspaceID, "w1"); err != nil {
		t.Fatalf("Set(workspace) error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var cfg fileConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if cfg.Context.ProjectID != "p1" || cfg.Context.WorkspaceID != "w1" {
		t.Errorf("saved context = %+v, want project=p1 workspace=w1 (Set should merge, not clobber)", cfg.Context)
	}
}

func TestSet_UnknownKey(t *testing.T) {
	tempConfigPath(t)
	if err := Set("bogus.key", "v"); err == nil {
		t.Error("Set(unknown key) error = nil, want error")
	}
}

func TestSet_PreservesAPIBaseField(t *testing.T) {
	path := tempConfigPath(t)
	if err := os.WriteFile(path, []byte("api_base = \"https://custom.example\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := Set(KeyProjectID, "p1"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var cfg fileConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if cfg.APIBase != "https://custom.example" {
		t.Errorf("APIBase = %q, want it preserved across Set() (Set must not clobber unrelated fields)", cfg.APIBase)
	}
	if cfg.Context.ProjectID != "p1" {
		t.Errorf("ProjectID = %q, want %q", cfg.Context.ProjectID, "p1")
	}
}

func TestSet_CorruptedExistingFile(t *testing.T) {
	path := tempConfigPath(t)
	if err := os.WriteFile(path, []byte("not valid toml [[["), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := Set(KeyProjectID, "p1"); err == nil {
		t.Error("Set() with corrupted config file: error = nil, want error")
	}
}

func TestClear(t *testing.T) {
	path := tempConfigPath(t)

	if err := Clear(); err != nil {
		t.Fatalf("Clear() on missing file error = %v, want nil", err)
	}

	if err := Set(KeyProjectID, "p1"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if err := Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("config file still exists after Clear(): err = %v", err)
	}
}
