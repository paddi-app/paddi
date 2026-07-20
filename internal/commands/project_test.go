package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
)

func TestChooseProject_TooManyArgs(t *testing.T) {
	cmd := parsedCmd(t, "switch", nil, "a", "b")
	if _, err := chooseProject(context.Background(), cmd); err == nil {
		t.Error("chooseProject() error = nil, want usage error")
	}
}

func TestChooseProject_KnownIDAdoptsItsWorkspace(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1", Name: "Website", WorkspaceID: "w1"}})
	})
	cmd := parsedCmd(t, "switch", nil, "p1")

	got, err := chooseProject(context.Background(), cmd)
	if err != nil {
		t.Fatalf("chooseProject() error = %v", err)
	}
	if got.Name != "Website" || got.WorkspaceID != "w1" {
		t.Errorf("chooseProject() = %+v, want {Name: Website, WorkspaceID: w1}", got)
	}
}

func TestChooseProject_UnknownIDFallsBackToBareID(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1", Name: "Website"}})
	})
	cmd := parsedCmd(t, "switch", nil, "missing")

	got, err := chooseProject(context.Background(), cmd)
	if err != nil {
		t.Fatalf("chooseProject() error = %v", err)
	}
	if got.ID != "missing" || got.Name != "" {
		t.Errorf("chooseProject() = %+v, want bare {ID: missing}", got)
	}
}

func TestRunProjectList_Quiet(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1"}, {ID: "p2"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = true; o.JSON = false })
	out := captureStdout(t, func() {
		if err := runProjectList(context.Background(), nil); err != nil {
			t.Fatalf("runProjectList() error = %v", err)
		}
	})
	if out != "p1\np2\n" {
		t.Errorf("stdout = %q, want %q", out, "p1\np2\n")
	}
}

func TestRunProjectList_JSON(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = true; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runProjectList(context.Background(), nil); err != nil {
			t.Fatalf("runProjectList() error = %v", err)
		}
	})
	if !strings.Contains(out, `"id":"p1"`) {
		t.Errorf("stdout = %q, want raw JSON with id p1", out)
	}
}

func TestRunProjectList_TableTruncatesDescription(t *testing.T) {
	long := strings.Repeat("a", 100)
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1", Name: "Website", Description: long}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runProjectList(context.Background(), nil); err != nil {
			t.Fatalf("runProjectList() error = %v", err)
		}
	})
	if !strings.Contains(out, "…") || strings.Contains(out, long) {
		t.Errorf("stdout = %q, want the description truncated to 60 runes with an ellipsis", out)
	}
}

func TestRunProjectSwitch_NoArgsRequiresWorkspace(t *testing.T) {
	withOpts(t, func(o *cmdutil.Options) { o.WorkspaceID = "" })
	cmd := parsedCmd(t, "switch", nil)
	if err := runProjectSwitch(context.Background(), cmd); err == nil {
		t.Error("runProjectSwitch() error = nil, want a missing-workspace error")
	}
}

type projectContextFile struct {
	Context struct {
		ProjectID   string `toml:"project_id"`
		WorkspaceID string `toml:"workspace_id"`
	} `toml:"context"`
}

func readProjectContextFile(t *testing.T, path string) projectContextFile {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	var cfg projectContextFile
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	return cfg
}

func TestRunProjectSwitch_ExplicitIDAdoptsWorkspace(t *testing.T) {
	path := withConfigFile(t)
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1", Name: "Website", WorkspaceID: "w1"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = false })
	cmd := parsedCmd(t, "switch", nil, "p1")
	out := captureStdout(t, func() {
		if err := runProjectSwitch(context.Background(), cmd); err != nil {
			t.Fatalf("runProjectSwitch() error = %v", err)
		}
	})
	if out != "Project set to Website\n" {
		t.Errorf("stdout = %q, want %q", out, "Project set to Website\n")
	}
	cfg := readProjectContextFile(t, path)
	if cfg.Context.ProjectID != "p1" || cfg.Context.WorkspaceID != "w1" {
		t.Errorf("config = %+v, want project=p1 workspace=w1 (adopted from the resolved project)", cfg.Context)
	}
}

func TestRunProjectSwitch_UnknownIDDoesNotAdoptWorkspace(t *testing.T) {
	path := withConfigFile(t)
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{})
	})
	cmd := parsedCmd(t, "switch", nil, "missing")
	captureStdout(t, func() {
		if err := runProjectSwitch(context.Background(), cmd); err != nil {
			t.Fatalf("runProjectSwitch() error = %v", err)
		}
	})
	cfg := readProjectContextFile(t, path)
	if cfg.Context.ProjectID != "missing" || cfg.Context.WorkspaceID != "" {
		t.Errorf("config = %+v, want project=missing and workspace left unset", cfg.Context)
	}
}

func TestRunProjectSwitch_QuietSuppressesMessage(t *testing.T) {
	withConfigFile(t)
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Project{{ID: "p1", Name: "Website"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = true })
	cmd := parsedCmd(t, "switch", nil, "p1")
	out := captureStdout(t, func() {
		if err := runProjectSwitch(context.Background(), cmd); err != nil {
			t.Fatalf("runProjectSwitch() error = %v", err)
		}
	})
	if out != "" {
		t.Errorf("stdout = %q, want empty output in quiet mode", out)
	}
}
