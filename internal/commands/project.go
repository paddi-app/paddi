package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
	"github.com/paddi-app/paddi/internal/config"
	"github.com/paddi-app/paddi/internal/output"
	"github.com/paddi-app/paddi/internal/prompt"
)

var projectCommand = &cli.Command{
	Name:  "project",
	Usage: "Manage project context",
	Commands: []*cli.Command{
		{
			Name:      "create",
			Usage:     "Create a project in the current workspace and set it as the current one",
			ArgsUsage: "<name>",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "description", Aliases: []string{"d"}, Usage: "what this project is about"},
				&cli.StringFlag{Name: "product-type", Usage: "product type (" + strings.Join(productTypes, ", ") + ")"},
				&cli.StringFlag{Name: "product-stage", Usage: "product stage (" + strings.Join(productStages, ", ") + ")"},
				&cli.StringSliceFlag{Name: "target-audience", Usage: "who this project is for (repeatable)"},
				&cli.StringSliceFlag{Name: "excluded-audience", Usage: "who this project is not for (repeatable)"},
				&cli.StringSliceFlag{Name: "business-goal", Usage: "business goal, highest priority first (repeatable)"},
			},
			Action: runProjectCreate,
		},
		{Name: "list", Usage: "List projects in the current workspace", Action: runProjectList},
		{Name: "switch", Usage: "Set the current project", ArgsUsage: "[project-id]", Action: runProjectSwitch},
	},
}

func runProjectList(ctx context.Context, _ *cli.Command) error {
	client, err := newClient()
	if err != nil {
		return err
	}
	projects, raw, err := client.ListProjects(ctx, opts.WorkspaceID)
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	if opts.Quiet {
		for _, p := range projects {
			fmt.Println(p.ID)
		}
		return nil
	}
	rows := make([][]string, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, []string{p.ID, p.Name, output.Truncate(p.Description, 60)})
	}
	return output.Table(os.Stdout, []string{"ID", "NAME", "DESCRIPTION"}, rows)
}

// Enum values accepted by the backend (model.AllProjectProductTypes /
// model.AllProjectProductStages), same options the onboarding form offers.
var (
	productTypes  = []string{"b2b_saas", "b2c_app", "developer_tool", "internal_tool", "marketplace", "ecommerce", "content_media", "other"}
	productStages = []string{"pre_alpha", "alpha", "beta", "pmf_validation", "growth", "scale"}
)

// projectDescriptionMaxLength mirrors the onboarding form's limit on the
// "About this project" field.
const projectDescriptionMaxLength = 1000

func runProjectCreate(ctx context.Context, cmd *cli.Command) error {
	input, err := projectInput(cmd)
	if err != nil {
		return err
	}
	input.WorkspaceID, err = cmdutil.RequireWorkspace(opts.WorkspaceID)
	if err != nil {
		return err
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	project, raw, err := client.CreateProject(ctx, *input)
	if err != nil {
		return err
	}
	if err := config.Set(config.KeyProjectID, project.ID); err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	if opts.Quiet {
		fmt.Println(project.ID)
	} else {
		fmt.Printf("Project %s created and set as current.\n", project.Name)
	}
	return nil
}

func runProjectSwitch(ctx context.Context, cmd *cli.Command) error {
	proj, err := chooseProject(ctx, cmd)
	if err != nil || proj == nil {
		return err
	}
	if err := config.Set(config.KeyProjectID, proj.ID); err != nil {
		return err
	}
	// Adopt the project's workspace so the two stay in sync and subsequent
	// listings are scoped to it.
	if proj.WorkspaceID != "" {
		if err := config.Set(config.KeyWorkspaceID, proj.WorkspaceID); err != nil {
			return err
		}
	}
	if !opts.Quiet {
		name := proj.Name
		if name == "" {
			name = proj.ID
		}
		fmt.Printf("Project set to %s\n", name)
	}
	return nil
}

// projectInput validates the create flags and args into an API payload,
// applying the same field rules as the onboarding create-project form.
// WorkspaceID is left for the caller to fill in.
func projectInput(cmd *cli.Command) (*api.ProjectInput, error) {
	name, err := cmdutil.SingleArg(cmd, "paddi project create <name>")
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 50 || strings.ContainsAny(name, `/\<>`) {
		return nil, errors.New(`project name must be 1-50 characters and cannot contain /, \, < or >`)
	}
	description := cmd.String("description")
	if len(description) > projectDescriptionMaxLength {
		return nil, fmt.Errorf("description must be at most %d characters", projectDescriptionMaxLength)
	}
	productType, err := oneOf(cmd.String("product-type"), productTypes, "product-type")
	if err != nil {
		return nil, err
	}
	productStage, err := oneOf(cmd.String("product-stage"), productStages, "product-stage")
	if err != nil {
		return nil, err
	}
	return &api.ProjectInput{
		Name:             name,
		Description:      description,
		ProductType:      productType,
		ProductStage:     productStage,
		TargetAudience:   cmd.StringSlice("target-audience"),
		ExcludedAudience: cmd.StringSlice("excluded-audience"),
		BusinessGoals:    cmd.StringSlice("business-goal"),
	}, nil
}

// oneOf passes an empty value through (the field is optional) and otherwise
// checks it against the allowed enum values.
func oneOf(value string, allowed []string, flag string) (string, error) {
	if value == "" || slices.Contains(allowed, value) {
		return value, nil
	}
	return "", fmt.Errorf("invalid --%s %q: must be one of %s", flag, value, strings.Join(allowed, ", "))
}

// chooseProject resolves the selected project from an explicit argument or an
// interactive picker. The returned project carries its WorkspaceID so callers
// can adopt it as the current workspace. Returns (nil, nil) when the user
// cancels the picker.
func chooseProject(ctx context.Context, cmd *cli.Command) (*api.Project, error) {
	if cmd.Args().Len() > 1 {
		return nil, errors.New("usage: paddi project switch [project-id]")
	}

	client, err := newClient()
	if err != nil {
		return nil, err
	}

	// Explicit id: resolve across all workspaces (empty filter) so its
	// workspace can be adopted even when it differs from the current context.
	if id := cmd.Args().First(); id != "" {
		projects, _, err := client.ListProjects(ctx, "")
		if err != nil {
			return nil, err
		}
		for i := range projects {
			if projects[i].ID == id {
				return &projects[i], nil
			}
		}
		return &api.Project{ID: id}, nil
	}

	// No id: present a picker scoped to the current workspace.
	workspaceID, err := cmdutil.RequireWorkspace(opts.WorkspaceID)
	if err != nil {
		return nil, err
	}
	return pickProject(ctx, client, workspaceID)
}

// pickProject presents an interactive picker over the given workspace's
// projects. Returns (nil, nil) when the user cancels the picker.
func pickProject(ctx context.Context, client *api.Client, workspaceID string) (*api.Project, error) {
	projects, _, err := client.ListProjects(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if len(projects) == 0 {
		return nil, errors.New("no projects available in the current workspace")
	}

	items := make([]prompt.Item, 0, len(projects))
	for _, p := range projects {
		items = append(items, prompt.Item{ID: p.ID, Label: p.Name})
	}
	choice, err := prompt.Select("Select a project (↑/↓ or j/k to move, Enter to select, q to cancel):", items)
	if errors.Is(err, prompt.ErrCancelled) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	for i := range projects {
		if projects[i].ID == choice.ID {
			return &projects[i], nil
		}
	}
	return &api.Project{ID: choice.ID, Name: choice.Label}, nil
}
