package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
)

var captureFlags = captureCommand().Commands[1].Flags

func TestRunCaptureList_RequiresProject(t *testing.T) {
	withOpts(t, func(o *cmdutil.Options) { o.Project = "" })
	if err := runCaptureList(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "project") {
		t.Errorf("runCaptureList() error = %v, want a missing-project error", err)
	}
}

func TestRunCaptureList_Quiet(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Capture{{ID: "c1"}, {ID: "c2"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.Quiet = true; o.JSON = false })
	out := captureStdout(t, func() {
		if err := runCaptureList(context.Background(), nil); err != nil {
			t.Fatalf("runCaptureList() error = %v", err)
		}
	})
	if out != "c1\nc2\n" {
		t.Errorf("stdout = %q, want %q", out, "c1\nc2\n")
	}
}

func TestRunCaptureList_JSON(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Capture{{ID: "c1"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.JSON = true; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runCaptureList(context.Background(), nil); err != nil {
			t.Fatalf("runCaptureList() error = %v", err)
		}
	})
	if !strings.Contains(out, `"id":"c1"`) {
		t.Errorf("stdout = %q, want raw JSON with id c1", out)
	}
}

func TestRunCaptureList_TableFallsBackToDescription(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Capture{
			{ID: "c1", Description: "raw description", Status: "pending", CreatedAt: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)},
			{ID: "c2", Name: "Pretty Name", Status: "processed", CreatedAt: time.Date(2026, 1, 3, 3, 4, 0, 0, time.UTC)},
		})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.JSON = false; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runCaptureList(context.Background(), nil); err != nil {
			t.Fatalf("runCaptureList() error = %v", err)
		}
	})
	if !strings.Contains(out, "raw description") {
		t.Errorf("stdout = %q, want it to fall back to Description when Name is empty", out)
	}
	if !strings.Contains(out, "Pretty Name") || !strings.Contains(out, "processed") {
		t.Errorf("stdout = %q, want the populated row's Name and Status", out)
	}
}

func TestCaptureDescription(t *testing.T) {
	t.Run("message flag wins verbatim", func(t *testing.T) {
		cmd := parsedCmd(t, "create", captureFlags, "-m", "  it broke  ")
		got, err := captureDescription(cmd)
		if err != nil {
			t.Fatalf("captureDescription() error = %v", err)
		}
		if got != "  it broke  " {
			t.Errorf("captureDescription() = %q, want %q (message flag is not trimmed)", got, "  it broke  ")
		}
	})

	t.Run("file flag reads and trims file content", func(t *testing.T) {
		path := writeTemp(t, "  from file  \n")
		cmd := parsedCmd(t, "create", captureFlags, "-f", path)
		got, err := captureDescription(cmd)
		if err != nil {
			t.Fatalf("captureDescription() error = %v", err)
		}
		if got != "from file" {
			t.Errorf("captureDescription() = %q, want %q", got, "from file")
		}
	})

	t.Run("dash arg reads and trims stdin", func(t *testing.T) {
		withStdin(t, "from stdin\n")
		cmd := parsedCmd(t, "create", captureFlags, "-")
		got, err := captureDescription(cmd)
		if err != nil {
			t.Fatalf("captureDescription() error = %v", err)
		}
		if got != "from stdin" {
			t.Errorf("captureDescription() = %q, want %q", got, "from stdin")
		}
	})

	t.Run("whitespace-only file content errors", func(t *testing.T) {
		path := writeTemp(t, "   \n")
		cmd := parsedCmd(t, "create", captureFlags, "-f", path)
		if _, err := captureDescription(cmd); err == nil {
			t.Error("captureDescription() error = nil, want empty-feedback error")
		}
	})

	t.Run("no message, file, or dash arg errors", func(t *testing.T) {
		cmd := parsedCmd(t, "create", captureFlags)
		if _, err := captureDescription(cmd); err == nil {
			t.Error("captureDescription() error = nil, want usage error")
		}
	})

	t.Run("message flag takes priority over file flag", func(t *testing.T) {
		path := writeTemp(t, "from file")
		cmd := parsedCmd(t, "create", captureFlags, "-m", "from message", "-f", path)
		got, err := captureDescription(cmd)
		if err != nil {
			t.Fatalf("captureDescription() error = %v", err)
		}
		if got != "from message" {
			t.Errorf("captureDescription() = %q, want %q (message flag should win)", got, "from message")
		}
	})
}

func TestRunCaptureCreate_DescriptionCheckedBeforeProjectContext(t *testing.T) {
	withOpts(t, func(o *cmdutil.Options) { o.Project = "" })
	cmd := parsedCmd(t, "create", captureFlags)
	err := runCaptureCreate(context.Background(), cmd)
	if err == nil || strings.Contains(err.Error(), "project") {
		t.Errorf("runCaptureCreate() error = %v, want the missing-feedback error (checked before project context)", err)
	}
}

func TestRunCaptureCreate_RequiresProject(t *testing.T) {
	withOpts(t, func(o *cmdutil.Options) { o.Project = "" })
	cmd := parsedCmd(t, "create", captureFlags, "-m", "it broke")
	err := runCaptureCreate(context.Background(), cmd)
	if err == nil || !strings.Contains(err.Error(), "project") {
		t.Errorf("runCaptureCreate() error = %v, want a missing-project error", err)
	}
}

func TestRunCaptureCreate_Success(t *testing.T) {
	var gotBody api.CaptureInput
	withAPIBase(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "c1"})
	})

	t.Run("normal output", func(t *testing.T) {
		withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.JSON = false; o.Quiet = false })
		cmd := parsedCmd(t, "create", captureFlags, "-m", "it broke", "--origin", "o1", "--tag", "bug")
		out := captureStdout(t, func() {
			if err := runCaptureCreate(context.Background(), cmd); err != nil {
				t.Fatalf("runCaptureCreate() error = %v", err)
			}
		})
		if out != "Capture c1 created.\n" {
			t.Errorf("stdout = %q, want %q", out, "Capture c1 created.\n")
		}
		if gotBody.ProjectID != "p1" || gotBody.Description != "it broke" || gotBody.OriginID != "o1" {
			t.Errorf("request body = %+v, want project p1, description %q, origin o1", gotBody, "it broke")
		}
	})

	t.Run("quiet prints only the id", func(t *testing.T) {
		withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.JSON = false; o.Quiet = true })
		cmd := parsedCmd(t, "create", captureFlags, "-m", "it broke")
		out := captureStdout(t, func() {
			if err := runCaptureCreate(context.Background(), cmd); err != nil {
				t.Fatalf("runCaptureCreate() error = %v", err)
			}
		})
		if out != "c1\n" {
			t.Errorf("stdout = %q, want %q", out, "c1\n")
		}
	})

	t.Run("json passes through raw response", func(t *testing.T) {
		withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.JSON = true; o.Quiet = false })
		cmd := parsedCmd(t, "create", captureFlags, "-m", "it broke")
		out := captureStdout(t, func() {
			if err := runCaptureCreate(context.Background(), cmd); err != nil {
				t.Fatalf("runCaptureCreate() error = %v", err)
			}
		})
		if !strings.Contains(out, `"id":"c1"`) {
			t.Errorf("stdout = %q, want it to contain the raw JSON id field", out)
		}
	})
}

func TestRunCaptureCreate_PropagatesAPIError(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1" })
	cmd := parsedCmd(t, "create", captureFlags, "-m", "it broke")
	if err := runCaptureCreate(context.Background(), cmd); err == nil {
		t.Error("runCaptureCreate() error = nil, want the server's 500 to propagate")
	}
}
