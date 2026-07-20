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

func TestRunSourceList_RequiresProject(t *testing.T) {
	withOpts(t, func(o *cmdutil.Options) { o.Project = "" })
	if err := runSourceList(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "project") {
		t.Errorf("runSourceList() error = %v, want a missing-project error", err)
	}
}

func TestRunSourceList_Quiet(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Source{{ID: "s1"}, {ID: "s2"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.Quiet = true; o.JSON = false })
	out := captureStdout(t, func() {
		if err := runSourceList(context.Background(), nil); err != nil {
			t.Fatalf("runSourceList() error = %v", err)
		}
	})
	if out != "s1\ns2\n" {
		t.Errorf("stdout = %q, want %q", out, "s1\ns2\n")
	}
}

func TestRunSourceList_JSON(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Source{{ID: "s1"}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.JSON = true; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runSourceList(context.Background(), nil); err != nil {
			t.Fatalf("runSourceList() error = %v", err)
		}
	})
	if !strings.Contains(out, `"id":"s1"`) {
		t.Errorf("stdout = %q, want raw JSON with id s1", out)
	}
}

func TestRunSourceList_TableFallbacks(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Source{
			{ID: "s1", Subject: "raw-subject"}, // no DisplayName, no LookupStatus, never indexed
			{ID: "s2", DisplayName: "Pretty Name", LookupStatus: "ready", LastIndexedAt: time.Date(2026, 1, 2, 3, 4, 0, 0, time.UTC)},
		})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.JSON = false; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runSourceList(context.Background(), nil); err != nil {
			t.Fatalf("runSourceList() error = %v", err)
		}
	})
	if !strings.Contains(out, "raw-subject") {
		t.Errorf("stdout = %q, want it to fall back to Subject when DisplayName is empty", out)
	}
	if !strings.Contains(out, "never") {
		t.Errorf("stdout = %q, want \"never\" for a zero LastIndexedAt", out)
	}
	if !strings.Contains(out, "-") {
		t.Errorf("stdout = %q, want \"-\" for an empty LookupStatus", out)
	}
	if !strings.Contains(out, "Pretty Name") || !strings.Contains(out, "ready") {
		t.Errorf("stdout = %q, want the populated row's DisplayName and LookupStatus", out)
	}
}

func TestRunSourceIndex_UsageErrorWithZeroArgs(t *testing.T) {
	cmd := parsedCmd(t, "index", nil)
	if err := runSourceIndex(context.Background(), cmd); err == nil {
		t.Error("runSourceIndex() error = nil, want usage error with zero args")
	}
}

func TestRunSourceIndex_Success(t *testing.T) {
	var gotPath string
	withAPIBase(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = false })
	cmd := parsedCmd(t, "index", nil, "src1")
	out := captureStdout(t, func() {
		if err := runSourceIndex(context.Background(), cmd); err != nil {
			t.Fatalf("runSourceIndex() error = %v", err)
		}
	})
	if gotPath != "/source/src1/index" {
		t.Errorf("path = %q, want /source/src1/index", gotPath)
	}
	if out != "Indexing triggered for source src1\n" {
		t.Errorf("stdout = %q, want %q", out, "Indexing triggered for source src1\n")
	}
}

func TestRunSourceIndex_QuietSuppressesMessage(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	withOpts(t, func(o *cmdutil.Options) { o.Quiet = true })
	cmd := parsedCmd(t, "index", nil, "src1")
	out := captureStdout(t, func() {
		if err := runSourceIndex(context.Background(), cmd); err != nil {
			t.Fatalf("runSourceIndex() error = %v", err)
		}
	})
	if out != "" {
		t.Errorf("stdout = %q, want empty output in quiet mode", out)
	}
}

func TestRunSourceIndex_PropagatesAPIError(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	cmd := parsedCmd(t, "index", nil, "src1")
	if err := runSourceIndex(context.Background(), cmd); err == nil {
		t.Error("runSourceIndex() error = nil, want the server's 500 to propagate")
	}
}
