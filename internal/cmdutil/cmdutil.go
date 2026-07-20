// Package cmdutil holds the command-runtime plumbing shared by every command
// group: config/client construction, context requirements, and small input
// helpers. It has no cli.Command definitions of its own.
package cmdutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/api"
)

// Options holds the global flags shared by every command. It is populated
// directly by cli.Command flag parsing (flag > env > config file > default).
type Options struct {
	JSON        bool
	Quiet       bool
	Project     string
	APIBase     string
	WorkspaceID string
}

// RequireProject returns the current project id, or an error telling the
// user how to set one.
func RequireProject(id string) (string, error) {
	return requireContext(id, "project", "or pass --project")
}

// RequireWorkspace returns the current workspace id, or an error telling the
// user how to set one.
func RequireWorkspace(id string) (string, error) {
	return requireContext(id, "workspace", "")
}

// requireContext returns id, or an error telling the user to run
// `paddi <kind> switch` first. altHint, if set, is offered as a shortcut
// alternative to switching (e.g. "or pass --project").
func requireContext(id, kind, altHint string) (string, error) {
	if id != "" {
		return id, nil
	}
	msg := fmt.Sprintf("no %s selected: run `paddi %s switch` first", kind, kind)
	if altHint != "" {
		msg = fmt.Sprintf("no %s selected: run `paddi %s switch`, or %s", kind, kind, altHint)
	}
	return "", errors.New(msg)
}

// SingleArg extracts exactly one positional argument, or an error showing usage.
func SingleArg(cmd *cli.Command, usage string) (string, error) {
	if cmd.Args().Len() != 1 {
		return "", errors.New("usage: " + usage)
	}
	return cmd.Args().First(), nil
}

// ResolveContextNames maps the current workspace/project IDs to their display
// names via a single workspace listing (whose entries carry nested projects).
// It is best-effort: a failed lookup falls back to the raw ID so name
// resolution never breaks `auth status`.
func ResolveContextNames(ctx context.Context, client *api.Client, workspaceID, projectID string) (workspace, project string) {
	workspace, project = workspaceID, projectID
	if workspaceID == "" && projectID == "" {
		return workspace, project
	}
	workspaces, _, err := client.ListWorkspaces(ctx)
	if err != nil {
		return workspace, project
	}
	for _, w := range workspaces {
		if w.ID == workspaceID {
			workspace = w.Name
		}
		for i := range w.Projects {
			if w.Projects[i].ID == projectID {
				project = w.Projects[i].Name
			}
		}
	}
	return workspace, project
}

// ReadFileOrStdin reads path, or stdin when path is "-".
func ReadFileOrStdin(path string) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}
