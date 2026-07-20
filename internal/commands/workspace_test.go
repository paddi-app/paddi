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

func TestChooseWorkspace_TooManyArgs(t *testing.T) {
	cmd := parsedCmd(t, "switch", nil, "a", "b")
	if _, _, err := chooseWorkspace(context.Background(), cmd); err == nil {
		t.Error("chooseWorkspace() error = nil, want usage error")
	}
}

func TestChooseWorkspace_ExplicitID(t *testing.T) {
	cmd := parsedCmd(t, "switch", nil, "w1")
	id, name, err := chooseWorkspace(context.Background(), cmd)
	if err != nil {
		t.Fatalf("chooseWorkspace() error = %v", err)
	}
	if id != "w1" || name != "" {
		t.Errorf("chooseWorkspace() = (%q, %q), want (\"w1\", \"\") (name is only resolved via the picker)", id, name)
	}
}

func TestChooseWorkspace_NoneAvailable(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Workspace{})
	})
	cmd := parsedCmd(t, "switch", nil)
	if _, _, err := chooseWorkspace(context.Background(), cmd); err == nil {
		t.Error("chooseWorkspace() error = nil, want 'no workspaces available' error")
	}
}

func TestRunWorkspaceList_Quiet(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Workspace{{ID: "w1"}, {ID: "w2"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = true; o.JSON = false })
	out := captureStdout(t, func() {
		if err := runWorkspaceList(context.Background(), nil); err != nil {
			t.Fatalf("runWorkspaceList() error = %v", err)
		}
	})
	if out != "w1\nw2\n" {
		t.Errorf("stdout = %q, want %q", out, "w1\nw2\n")
	}
}

func TestRunWorkspaceList_JSON(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Workspace{{ID: "w1"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = true; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runWorkspaceList(context.Background(), nil); err != nil {
			t.Fatalf("runWorkspaceList() error = %v", err)
		}
	})
	if !strings.Contains(out, `"id":"w1"`) {
		t.Errorf("stdout = %q, want raw JSON with id w1", out)
	}
}

func TestRunWorkspaceList_TableShowsRoleAndProjectCount(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Workspace{
			{ID: "w1", Name: "Acme", Member: &api.WorkspaceMember{Role: api.WorkspaceMemberRoleOwner}, Projects: []api.Project{{ID: "p1"}, {ID: "p2"}}},
			{ID: "w2", Name: "NoRole"},
		})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runWorkspaceList(context.Background(), nil); err != nil {
			t.Fatalf("runWorkspaceList() error = %v", err)
		}
	})
	if !strings.Contains(out, "Owner") {
		t.Errorf("stdout = %q, want it to show the Owner role", out)
	}
	if !strings.Contains(out, "w1") || !strings.Contains(out, "2") {
		t.Errorf("stdout = %q, want the project count (2) for w1", out)
	}
	w2Line := ""
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "w2") {
			w2Line = line
		}
	}
	if w2Line == "" || strings.Contains(w2Line, "Owner") {
		t.Errorf("w2 row = %q, want an empty role (no WorkspaceMember set)", w2Line)
	}
}

func TestRunWorkspaceSwitch_NoneAvailable(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Workspace{})
	})
	cmd := parsedCmd(t, "switch", nil)
	if err := runWorkspaceSwitch(context.Background(), cmd); err == nil {
		t.Error("runWorkspaceSwitch() error = nil, want 'no workspaces available' error")
	}
}

type workspaceContextFile struct {
	Context struct {
		WorkspaceID string `toml:"workspace_id"`
	} `toml:"context"`
}

func TestRunWorkspaceSwitch_ExplicitIDWritesConfig(t *testing.T) {
	path := withConfigFile(t)
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = false })
	cmd := parsedCmd(t, "switch", nil, "w1")
	out := captureStdout(t, func() {
		if err := runWorkspaceSwitch(context.Background(), cmd); err != nil {
			t.Fatalf("runWorkspaceSwitch() error = %v", err)
		}
	})
	if out != "Workspace set to w1\n" {
		t.Errorf("stdout = %q, want %q (falls back to id when no name resolved)", out, "Workspace set to w1\n")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var cfg workspaceContextFile
	if err := toml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if cfg.Context.WorkspaceID != "w1" {
		t.Errorf("config workspace_id = %q, want %q", cfg.Context.WorkspaceID, "w1")
	}
}

func TestRunWorkspaceSwitch_QuietSuppressesMessage(t *testing.T) {
	withConfigFile(t)
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = true })
	cmd := parsedCmd(t, "switch", nil, "w1")
	out := captureStdout(t, func() {
		if err := runWorkspaceSwitch(context.Background(), cmd); err != nil {
			t.Fatalf("runWorkspaceSwitch() error = %v", err)
		}
	})
	if out != "" {
		t.Errorf("stdout = %q, want empty output in quiet mode", out)
	}
}
