package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
)

// The group's Before hook must reject id-based subcommands (which never checked
// the project themselves) before they reach the network.
func TestSpecCommand_RequiresProjectBeforeNetwork(t *testing.T) {
	withOpts(t, func(o *cmdutil.Options) { o.Project = "" })
	err := specCommand().Run(context.Background(), []string{"spec", "view", "some-id"})
	if err == nil || !strings.Contains(err.Error(), "project") {
		t.Errorf("spec view without project: error = %v, want a missing-project error", err)
	}
}

func TestRunSpecList_SortsByCreatedAtDescending(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Spec{
			{ID: "old", CreatedAt: older},
			{ID: "new", CreatedAt: newer},
		})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.Quiet = true; o.JSON = false })
	out := captureStdout(t, func() {
		if err := runSpecList(context.Background(), nil); err != nil {
			t.Fatalf("runSpecList() error = %v", err)
		}
	})
	if out != "new\nold\n" {
		t.Errorf("stdout = %q, want the newest spec first", out)
	}
}

func TestRunSpecList_JSONBypassesSortAndReturnsRawServerOrder(t *testing.T) {
	older := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Spec{
			{ID: "old", CreatedAt: older},
			{ID: "new", CreatedAt: newer},
		})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.JSON = true; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runSpecList(context.Background(), nil); err != nil {
			t.Fatalf("runSpecList() error = %v", err)
		}
	})
	oldIdx, newIdx := strings.Index(out, `"old"`), strings.Index(out, `"new"`)
	if oldIdx == -1 || newIdx == -1 || oldIdx > newIdx {
		t.Errorf("stdout = %q, want raw server order preserved (old before new) since JSON output bypasses the sort", out)
	}
}

func TestRunSpecList_TableShowsRequestTypeAndLockedFlag(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Spec{
			{ID: "s1", Title: "Locked one", Locked: true, Request: &api.Request{Type: "bug"}},
			{ID: "s2", Title: "Unlocked one", Locked: false},
		})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.JSON = false; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runSpecList(context.Background(), nil); err != nil {
			t.Fatalf("runSpecList() error = %v", err)
		}
	})
	if !strings.Contains(out, "bug") {
		t.Errorf("stdout = %q, want the request type from the nested Request", out)
	}
	if !strings.Contains(out, "yes") || !strings.Contains(out, "no") {
		t.Errorf("stdout = %q, want both locked=yes and locked=no rows", out)
	}
}

func TestRunSpecInfo_UsageErrorWithZeroArgs(t *testing.T) {
	cmd := parsedCmd(t, "info", nil)
	if err := runSpecInfo(context.Background(), cmd); err == nil {
		t.Error("runSpecInfo() error = nil, want usage error with zero args")
	}
}

func TestRunSpecInfo_JSONStripsContent(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Spec{ID: "s1", Title: "My Spec", Content: "# secret body"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = true })
	cmd := parsedCmd(t, "info", nil, "s1")
	out := captureStdout(t, func() {
		if err := runSpecInfo(context.Background(), cmd); err != nil {
			t.Fatalf("runSpecInfo() error = %v", err)
		}
	})
	if strings.Contains(out, "secret body") {
		t.Errorf("stdout = %q, want the content field stripped from JSON info output", out)
	}
	if !strings.Contains(out, `"id":"s1"`) {
		t.Errorf("stdout = %q, want the id still present", out)
	}
}

func TestRunSpecInfo_FormattedOutputShowsLocked(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Spec{ID: "s1", Title: "My Spec", Locked: true})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false })
	cmd := parsedCmd(t, "info", nil, "s1")
	out := captureStdout(t, func() {
		if err := runSpecInfo(context.Background(), cmd); err != nil {
			t.Fatalf("runSpecInfo() error = %v", err)
		}
	})
	if !strings.Contains(out, "Locked: yes") {
		t.Errorf("stdout = %q, want it to report Locked: yes", out)
	}
}

func TestRunSpecView_JSON(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Spec{ID: "s1", Title: "My Spec", Content: "body"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = true })
	cmd := parsedCmd(t, "view", nil, "s1")
	out := captureStdout(t, func() {
		if err := runSpecView(context.Background(), cmd); err != nil {
			t.Fatalf("runSpecView() error = %v", err)
		}
	})
	if !strings.Contains(out, `"content":"body"`) {
		t.Errorf("stdout = %q, want the raw JSON including content", out)
	}
}

func TestRunSpecView_AppendsNewlineWhenContentLacksOne(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Spec{ID: "s1", Title: "My Spec", Content: "body without trailing newline"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false })
	cmd := parsedCmd(t, "view", nil, "s1")
	out := captureStdout(t, func() {
		if err := runSpecView(context.Background(), cmd); err != nil {
			t.Fatalf("runSpecView() error = %v", err)
		}
	})
	want := "# My Spec\nbody without trailing newline\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestRunSpecView_DoesNotDoubleExistingTrailingNewline(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Spec{ID: "s1", Title: "My Spec", Content: "body with newline\n"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false })
	cmd := parsedCmd(t, "view", nil, "s1")
	out := captureStdout(t, func() {
		if err := runSpecView(context.Background(), cmd); err != nil {
			t.Fatalf("runSpecView() error = %v", err)
		}
	})
	want := "# My Spec\nbody with newline\n"
	if out != want {
		t.Errorf("stdout = %q, want %q (no doubled newline)", out, want)
	}
}

func TestRunSpecDownload_UsesExplicitOutputPath(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Spec{ID: "s1", Title: "My Spec", Content: "spec body"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = false })
	dest := filepath.Join(t.TempDir(), "out.md")
	cmd := parsedCmd(t, "download", specCommand().Commands[3].Flags, "s1", "-o", dest)
	out := captureStdout(t, func() {
		if err := runSpecDownload(context.Background(), cmd); err != nil {
			t.Fatalf("runSpecDownload() error = %v", err)
		}
	})
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "spec body" {
		t.Errorf("file content = %q, want %q", data, "spec body")
	}
	if !strings.Contains(out, dest) {
		t.Errorf("stdout = %q, want it to mention the output path %q", out, dest)
	}
}

func TestRunSpecDownload_FallsBackToSpecFilename(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Spec{ID: "s1", Title: "My Spec", Content: "spec body"})
	})
	t.Chdir(t.TempDir())
	cmd := parsedCmd(t, "download", specCommand().Commands[3].Flags, "s1")
	captureStdout(t, func() {
		if err := runSpecDownload(context.Background(), cmd); err != nil {
			t.Fatalf("runSpecDownload() error = %v", err)
		}
	})
	if _, err := os.Stat("My Spec.md"); err != nil {
		t.Errorf("expected file %q to exist, stat error = %v", "My Spec.md", err)
	}
}

func TestRunSpecDownload_QuietPrintsOnlyThePath(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Spec{ID: "s1", Title: "My Spec", Content: "spec body"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = true })
	dest := filepath.Join(t.TempDir(), "out.md")
	cmd := parsedCmd(t, "download", specCommand().Commands[3].Flags, "s1", "-o", dest)
	out := captureStdout(t, func() {
		if err := runSpecDownload(context.Background(), cmd); err != nil {
			t.Fatalf("runSpecDownload() error = %v", err)
		}
	})
	if out != dest+"\n" {
		t.Errorf("stdout = %q, want just the path %q", out, dest+"\n")
	}
}

func TestRunSpecLock_Success(t *testing.T) {
	var gotMethod string
	withAPIBase(t, func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		_ = json.NewEncoder(w).Encode(api.Spec{ID: "s1", Title: "My Spec", Locked: true})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false; o.Quiet = false })
	cmd := parsedCmd(t, "lock", nil, "s1")
	out := captureStdout(t, func() {
		if err := runSpecLock(context.Background(), cmd); err != nil {
			t.Fatalf("runSpecLock() error = %v", err)
		}
	})
	if gotMethod != http.MethodPatch {
		t.Errorf("method = %q, want PATCH", gotMethod)
	}
	if out != "Spec \"My Spec\" locked.\n" {
		t.Errorf("stdout = %q, want %q", out, "Spec \"My Spec\" locked.\n")
	}
}

func TestRunSpecLock_QuietSuppressesMessage(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Spec{ID: "s1", Title: "My Spec"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false; o.Quiet = true })
	cmd := parsedCmd(t, "lock", nil, "s1")
	out := captureStdout(t, func() {
		if err := runSpecLock(context.Background(), cmd); err != nil {
			t.Fatalf("runSpecLock() error = %v", err)
		}
	})
	if out != "" {
		t.Errorf("stdout = %q, want empty output in quiet mode", out)
	}
}

func TestRunSpecLock_PropagatesAPIError(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	cmd := parsedCmd(t, "lock", nil, "s1")
	if err := runSpecLock(context.Background(), cmd); err == nil {
		t.Error("runSpecLock() error = nil, want the server's 500 to propagate")
	}
}
