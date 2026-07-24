package commands

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
)

func TestReadAnswers(t *testing.T) {
	cases := []struct {
		name    string
		content string
		wantErr bool
		wantLen int
	}{
		{"bare array", `[{"solution_path_id":"sp1","selections":[{"label":"A"}]}]`, false, 1},
		{"wrapped object", `{"answers":[{"solution_path_id":"sp1","selections":[]},{"solution_path_id":"sp2","selections":[]}]}`, false, 2},
		{"leading whitespace before array", "  \n[{\"solution_path_id\":\"sp1\",\"selections\":[]}]", false, 1},
		{"invalid json", `not json`, true, 0},
		{"empty array errors", `[]`, true, 0},
		{"wrapped object with no answers errors", `{}`, true, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "answers.json")
			if err := os.WriteFile(path, []byte(tc.content), 0o600); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}
			got, err := readAnswers(path)
			if tc.wantErr {
				if err == nil {
					t.Fatal("readAnswers() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("readAnswers() error = %v", err)
			}
			if len(got) != tc.wantLen {
				t.Errorf("readAnswers() len = %d, want %d", len(got), tc.wantLen)
			}
		})
	}
}

// The group's Before hook must reject id-based subcommands (which never checked
// the project themselves) before they reach the network.
func TestRequestCommand_RequiresProjectBeforeNetwork(t *testing.T) {
	withOpts(t, func(o *cmdutil.Options) { o.Project = "" })
	err := requestCommand().Run(context.Background(), []string{"request", "view", "some-id"})
	if err == nil || !strings.Contains(err.Error(), "project") {
		t.Errorf("request view without project: error = %v, want a missing-project error", err)
	}
}

func TestRunRequestList_SortsByScoreDescending(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Request{
			{ID: "low", Score: 1},
			{ID: "high", Score: 9},
			{ID: "mid", Score: 5},
		})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.Quiet = true; o.JSON = false })
	out := captureStdout(t, func() {
		if err := runRequestList(context.Background(), nil); err != nil {
			t.Fatalf("runRequestList() error = %v", err)
		}
	})
	if out != "high\nmid\nlow\n" {
		t.Errorf("stdout = %q, want ids sorted by descending score", out)
	}
}

func TestRunRequestList_JSONBypassesSortAndReturnsRawServerOrder(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Request{{ID: "low", Score: 1}, {ID: "high", Score: 9}})
	})
	withOpts(t, func(o *cmdutil.Options) { o.Project = "p1"; o.JSON = true; o.Quiet = false })
	out := captureStdout(t, func() {
		if err := runRequestList(context.Background(), nil); err != nil {
			t.Fatalf("runRequestList() error = %v", err)
		}
	})
	lowIdx, highIdx := strings.Index(out, `"low"`), strings.Index(out, `"high"`)
	if lowIdx == -1 || highIdx == -1 || lowIdx > highIdx {
		t.Errorf("stdout = %q, want raw server order preserved (low before high) since JSON output bypasses the sort", out)
	}
}

func TestRunRequestView_UsageErrorWithZeroArgs(t *testing.T) {
	cmd := parsedCmd(t, "view", nil)
	if err := runRequestView(context.Background(), cmd); err == nil {
		t.Error("runRequestView() error = nil, want usage error with zero args")
	}
}

func TestRunRequestView_JSON(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Request{ID: "r1", Name: "Fix bug"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = true })
	cmd := parsedCmd(t, "view", nil, "r1")
	out := captureStdout(t, func() {
		if err := runRequestView(context.Background(), cmd); err != nil {
			t.Fatalf("runRequestView() error = %v", err)
		}
	})
	if !strings.Contains(out, `"id":"r1"`) {
		t.Errorf("stdout = %q, want raw JSON with id r1", out)
	}
}

func TestRunRequestView_FormattedOutput(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Request{ID: "r1", Name: "Fix bug", Type: "bug", Status: "open"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false })
	cmd := parsedCmd(t, "view", nil, "r1")
	out := captureStdout(t, func() {
		if err := runRequestView(context.Background(), cmd); err != nil {
			t.Fatalf("runRequestView() error = %v", err)
		}
	})
	if !strings.HasPrefix(out, "Fix bug (r1)\n") {
		t.Errorf("stdout = %q, want it to start with the formatted request header", out)
	}
}

var requestRegenerateFlags = requestCommand().Commands[2].Flags

func TestRunRequestRegenerate_UsageErrorWithZeroArgs(t *testing.T) {
	cmd := parsedCmd(t, "regenerate", requestRegenerateFlags)
	if err := runRequestRegenerate(context.Background(), cmd); err == nil {
		t.Error("runRequestRegenerate() error = nil, want usage error with zero args")
	}
}

func TestRunRequestRegenerate_Success(t *testing.T) {
	var gotBody map[string]string
	withAPIBase(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(api.Request{ID: "r1", Name: "Fix bug", Status: "regenerating"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false; o.Quiet = false })
	cmd := parsedCmd(t, "regenerate", requestRegenerateFlags, "r1", "-e", "faster please")
	out := captureStdout(t, func() {
		if err := runRequestRegenerate(context.Background(), cmd); err != nil {
			t.Fatalf("runRequestRegenerate() error = %v", err)
		}
	})
	if !strings.Contains(out, "Fix bug") || !strings.Contains(out, "regenerating") {
		t.Errorf("stdout = %q, want it to mention the request name and status", out)
	}
	if gotBody["expectation"] != "faster please" {
		t.Errorf("body.expectation = %q, want %q", gotBody["expectation"], "faster please")
	}
}

func TestRunRequestRegenerate_QuietSuppressesMessage(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.Request{ID: "r1"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false; o.Quiet = true })
	cmd := parsedCmd(t, "regenerate", requestRegenerateFlags, "r1")
	out := captureStdout(t, func() {
		if err := runRequestRegenerate(context.Background(), cmd); err != nil {
			t.Fatalf("runRequestRegenerate() error = %v", err)
		}
	})
	if out != "" {
		t.Errorf("stdout = %q, want empty output in quiet mode", out)
	}
}

var requestDraftFlags = requestCommand().Commands[3].Flags

func TestRunRequestDraft_UsageErrorWithZeroArgs(t *testing.T) {
	path := writeTemp(t, `[{"solution_path_id":"sp1","selections":[]}]`)
	cmd := parsedCmd(t, "draft", requestDraftFlags, "-f", path)
	if err := runRequestDraft(context.Background(), cmd); err == nil {
		t.Error("runRequestDraft() error = nil, want usage error with zero id args")
	}
}

func TestRunRequestDraft_PropagatesInvalidAnswersFile(t *testing.T) {
	path := writeTemp(t, "not json")
	cmd := parsedCmd(t, "draft", requestDraftFlags, "r1", "-f", path)
	if err := runRequestDraft(context.Background(), cmd); err == nil {
		t.Error("runRequestDraft() error = nil, want invalid-answers-JSON error")
	}
}

func TestRunRequestDraft_Success(t *testing.T) {
	var gotBody struct {
		Answers []api.Answer `json:"answers"`
	}
	withAPIBase(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(api.Request{ID: "r1", Name: "Fix bug", Status: "drafting"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false; o.Quiet = false })
	path := writeTemp(t, `[{"solution_path_id":"sp1","selections":[{"label":"A"}]}]`)
	cmd := parsedCmd(t, "draft", requestDraftFlags, "r1", "-f", path)
	out := captureStdout(t, func() {
		if err := runRequestDraft(context.Background(), cmd); err != nil {
			t.Fatalf("runRequestDraft() error = %v", err)
		}
	})
	if !strings.Contains(out, "Fix bug") || !strings.Contains(out, "drafting") {
		t.Errorf("stdout = %q, want it to mention the request name and status", out)
	}
	if len(gotBody.Answers) != 1 || gotBody.Answers[0].SolutionPathID != "sp1" {
		t.Errorf("request body = %+v, want one answer for sp1", gotBody.Answers)
	}
}
