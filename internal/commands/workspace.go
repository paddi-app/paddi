package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
	"github.com/paddi-app/paddi/internal/config"
	"github.com/paddi-app/paddi/internal/output"
	"github.com/paddi-app/paddi/internal/prompt"
)

var workspaceCommand = &cli.Command{
	Name:  "workspace",
	Usage: "Manage workspace context",
	Commands: []*cli.Command{
		{
			Name:      "create",
			Usage:     "Create a workspace and set it as the current one",
			ArgsUsage: "<name>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "language", Aliases: []string{"l"}, Usage: "workspace language (en, zh-TW)", Value: "en"},
				&cli.StringFlag{Name: "team-size", Usage: "team size (e.g. 2-5)"},
				&cli.StringFlag{Name: "job-role", Usage: "your job role in the workspace"},
			},
			Action: runWorkspaceCreate,
		},
		{Name: "list", Usage: "List my workspaces", Action: runWorkspaceList},
		{Name: "switch", Usage: "Set the current workspace", ArgsUsage: "[workspace-id]", Action: runWorkspaceSwitch},
	},
}

func runWorkspaceList(ctx context.Context, _ *cli.Command) error {
	client, err := newClient()
	if err != nil {
		return err
	}
	workspaces, raw, err := client.ListWorkspaces(ctx)
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	if opts.Quiet {
		for _, w := range workspaces {
			fmt.Println(w.ID)
		}
		return nil
	}
	rows := make([][]string, 0, len(workspaces))
	for _, w := range workspaces {
		role := ""
		if w.Member != nil {
			role = w.Member.Role.String()
		}
		rows = append(rows, []string{w.ID, w.Name, role, strconv.Itoa(len(w.Projects))})
	}
	return output.Table(os.Stdout, []string{"ID", "NAME", "ROLE", "PROJECTS"}, rows)
}

func runWorkspaceCreate(ctx context.Context, cmd *cli.Command) error {
	name, err := cmdutil.SingleArg(cmd, "paddi workspace create <name>")
	if err != nil {
		return err
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	workspace, raw, err := client.CreateWorkspace(ctx, api.WorkspaceInput{
		Name:     name,
		JobRole:  cmd.String("job-role"),
		TeamSize: cmd.String("team-size"),
		Language: cmd.String("language"),
	})
	if err != nil {
		return err
	}
	if err := config.Set(config.KeyWorkspaceID, workspace.ID); err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	if opts.Quiet {
		fmt.Println(workspace.ID)
	} else {
		fmt.Printf("Workspace %s created and set as current.\n", workspace.Name)
	}
	return nil
}

func runWorkspaceSwitch(ctx context.Context, cmd *cli.Command) error {
	id, name, err := chooseWorkspace(ctx, cmd)
	if err != nil || id == "" {
		return err
	}
	if err := config.Set(config.KeyWorkspaceID, id); err != nil {
		return err
	}
	if !opts.Quiet {
		if name == "" {
			name = id
		}
		fmt.Printf("Workspace set to %s\n", name)
	}
	return nil
}

// chooseWorkspace resolves the workspace id from an explicit argument, or by
// presenting an interactive picker when none is given. The second return value
// is the workspace name (empty when an id is passed directly). Both are empty
// (with a nil error) when the user cancels the picker.
func chooseWorkspace(ctx context.Context, cmd *cli.Command) (id, name string, err error) {
	switch cmd.Args().Len() {
	case 1:
		return cmd.Args().First(), "", nil
	case 0:
	default:
		return "", "", errors.New("usage: paddi workspace switch [workspace-id]")
	}

	client, err := newClient()
	if err != nil {
		return "", "", err
	}
	return pickWorkspace(ctx, client)
}

// pickWorkspace presents an interactive picker over all of the user's
// workspaces. The returned name is the workspace label. Both values are empty
// (with a nil error) when the user cancels the picker.
func pickWorkspace(ctx context.Context, client *api.Client) (id, name string, err error) {
	workspaces, _, err := client.ListWorkspaces(ctx)
	if err != nil {
		return "", "", err
	}
	if len(workspaces) == 0 {
		return "", "", errors.New("no workspaces available")
	}

	items := make([]prompt.Item, 0, len(workspaces))
	for _, w := range workspaces {
		items = append(items, prompt.Item{ID: w.ID, Label: w.Name})
	}
	choice, err := prompt.Select("Select a workspace (↑/↓ or j/k to move, Enter to select, q to cancel):", items)
	if errors.Is(err, prompt.ErrCancelled) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return choice.ID, choice.Label, nil
}
