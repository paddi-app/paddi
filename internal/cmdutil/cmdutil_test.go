package cmdutil

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/api"
)

func TestRequireProject(t *testing.T) {
	if got, err := RequireProject("p1"); err != nil || got != "p1" {
		t.Errorf("RequireProject(\"p1\") = (%q, %v), want (\"p1\", nil)", got, err)
	}
	if _, err := RequireProject(""); err == nil {
		t.Error("RequireProject(\"\") error = nil, want error")
	}
}

func TestRequireWorkspace(t *testing.T) {
	if got, err := RequireWorkspace("w1"); err != nil || got != "w1" {
		t.Errorf("RequireWorkspace(\"w1\") = (%q, %v), want (\"w1\", nil)", got, err)
	}
	if _, err := RequireWorkspace(""); err == nil {
		t.Error("RequireWorkspace(\"\") error = nil, want error")
	}
}

func TestSingleArg(t *testing.T) {
	newCmd := func(args ...string) *cli.Command {
		cmd := &cli.Command{
			Name:   "test",
			Action: func(context.Context, *cli.Command) error { return nil },
		}
		if err := cmd.Run(context.Background(), append([]string{"test"}, args...)); err != nil {
			t.Fatalf("cmd.Run() error = %v", err)
		}
		return cmd
	}

	if got, err := SingleArg(newCmd("spec-id"), "paddi spec view <id>"); err != nil || got != "spec-id" {
		t.Errorf("SingleArg() = (%q, %v), want (\"spec-id\", nil)", got, err)
	}
	if _, err := SingleArg(newCmd(), "paddi spec view <id>"); err == nil {
		t.Error("SingleArg() with zero args: error = nil, want usage error")
	}
	if _, err := SingleArg(newCmd("a", "b"), "paddi spec view <id>"); err == nil {
		t.Error("SingleArg() with two args: error = nil, want usage error")
	}
}

func TestReadFileOrStdin_File(t *testing.T) {
	path := filepath.Join(t.TempDir(), "in.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, err := ReadFileOrStdin(path)
	if err != nil {
		t.Fatalf("ReadFileOrStdin() error = %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("ReadFileOrStdin() = %q, want %q", got, "hello")
	}
}

func TestReadFileOrStdin_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.txt")
	if _, err := ReadFileOrStdin(path); err == nil {
		t.Error("ReadFileOrStdin() with missing file: error = nil, want error")
	}
}

func TestReadFileOrStdin_Stdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = orig })

	go func() {
		_, _ = w.WriteString("from stdin")
		w.Close()
	}()

	got, err := ReadFileOrStdin("-")
	if err != nil {
		t.Fatalf("ReadFileOrStdin(\"-\") error = %v", err)
	}
	if string(got) != "from stdin" {
		t.Errorf("ReadFileOrStdin(\"-\") = %q, want %q", got, "from stdin")
	}
}

func TestResolveContextNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]api.Workspace{
			{ID: "w1", Name: "Acme", Projects: []api.Project{{ID: "p1", Name: "Website"}}},
		})
	}))
	defer srv.Close()
	client := &api.Client{BaseURL: srv.URL}

	t.Run("resolves known ids to names", func(t *testing.T) {
		ws, proj := ResolveContextNames(context.Background(), client, "w1", "p1")
		if ws != "Acme" || proj != "Website" {
			t.Errorf("ResolveContextNames() = (%q, %q), want (\"Acme\", \"Website\")", ws, proj)
		}
	})

	t.Run("falls back to raw id when unmatched", func(t *testing.T) {
		ws, proj := ResolveContextNames(context.Background(), client, "unknown", "")
		if ws != "unknown" || proj != "" {
			t.Errorf("ResolveContextNames() = (%q, %q), want (\"unknown\", \"\")", ws, proj)
		}
	})

	t.Run("skips the API call entirely when both empty", func(t *testing.T) {
		ws, proj := ResolveContextNames(context.Background(), &api.Client{BaseURL: "http://127.0.0.1:0"}, "", "")
		if ws != "" || proj != "" {
			t.Errorf("ResolveContextNames() = (%q, %q), want (\"\", \"\")", ws, proj)
		}
	})

	t.Run("falls back to raw ids when the API call errors", func(t *testing.T) {
		badClient := &api.Client{BaseURL: "http://127.0.0.1:0"}
		ws, proj := ResolveContextNames(context.Background(), badClient, "w1", "p1")
		if ws != "w1" || proj != "p1" {
			t.Errorf("ResolveContextNames() = (%q, %q), want (\"w1\", \"p1\") on API error", ws, proj)
		}
	})

	t.Run("resolves only the id that matches, leaving the other as raw", func(t *testing.T) {
		ws, proj := ResolveContextNames(context.Background(), client, "w1", "unknown-project")
		if ws != "Acme" || proj != "unknown-project" {
			t.Errorf("ResolveContextNames() = (%q, %q), want (\"Acme\", \"unknown-project\")", ws, proj)
		}
	})
}

func TestRequireContext_HintText(t *testing.T) {
	_, err := RequireProject("")
	if !strings.Contains(err.Error(), "or pass --project") {
		t.Errorf("RequireProject(\"\") error = %q, want it to mention --project", err)
	}
	_, err = RequireWorkspace("")
	if strings.Contains(err.Error(), "--project") {
		t.Errorf("RequireWorkspace(\"\") error = %q, should not mention --project", err)
	}
}
