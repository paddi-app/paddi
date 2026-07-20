package commands

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/cmdutil"
	"github.com/paddi-app/paddi/internal/config"
)

// parsedCmd builds a *cli.Command with the given flags, parses args, and
// returns it for inspecting parsed flag/arg state — without invoking a real
// Action (which would reach the network or credentials).
func parsedCmd(t *testing.T, name string, flags []cli.Flag, args ...string) *cli.Command {
	t.Helper()
	cmd := &cli.Command{
		Name:   name,
		Flags:  flags,
		Action: func(context.Context, *cli.Command) error { return nil },
	}
	if err := cmd.Run(context.Background(), append([]string{name}, args...)); err != nil {
		t.Fatalf("cmd.Run(%v) error = %v", args, err)
	}
	return cmd
}

// withAPIBase points opts.APIBase at a fake backend and bypasses the keyring
// via PADDI_TOKEN, restoring both after the test.
func withAPIBase(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	t.Setenv("PADDI_TOKEN", "test-token")
	prev := opts.APIBase
	opts.APIBase = srv.URL
	t.Cleanup(func() { opts.APIBase = prev })
}

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func withStdin(t *testing.T, content string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = orig })
	go func() {
		_, _ = w.WriteString(content)
		w.Close()
	}()
}

// withOpts snapshots the package-level opts, lets mutate adjust it, and
// restores the snapshot after the test.
func withOpts(t *testing.T, mutate func(o *cmdutil.Options)) {
	t.Helper()
	prev := opts
	mutate(&opts)
	t.Cleanup(func() { opts = prev })
}

// withConfigFile points config.Path() at a fresh temp file for the duration
// of the test, so Set/Clear never touch the real user config.
func withConfigFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	t.Setenv(config.EnvConfigPath, path)
	return path
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	return captureFD(t, &os.Stdout, fn)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	return captureFD(t, &os.Stderr, fn)
}

func captureFD(t *testing.T, target **os.File, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	orig := *target
	*target = w
	fn()
	*target = orig
	w.Close()
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return string(out)
}
