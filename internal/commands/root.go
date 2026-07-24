package commands

import (
	"context"

	altsrc "github.com/urfave/cli-altsrc/v3"
	"github.com/urfave/cli-altsrc/v3/toml"
	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
	"github.com/paddi-app/paddi/internal/config"
)

var opts cmdutil.Options

// newClient builds an API client for the current API base.
func newClient() (*api.Client, error) {
	return api.NewClient(opts.APIBase)
}

// requireProject is a Before hook that fails a command group early if no
// project context is set. Actions that need the ID still call
// cmdutil.RequireProject themselves.
func requireProject(ctx context.Context, _ *cli.Command) (context.Context, error) {
	_, err := cmdutil.RequireProject(opts.Project)
	return ctx, err
}

// Root builds the paddi root command.
func Root() *cli.Command {
	opts = cmdutil.Options{}
	cli.VersionFlag = &cli.BoolFlag{
		Name:        "version",
		Aliases:     []string{"V"},
		Usage:       "print the version",
		HideDefault: true,
		Local:       true,
	}

	// Config file path is resolved once at startup; a missing/unresolvable
	// path just means the file source never matches (falls through to env
	// or default), matching the old missing-file behavior.
	configPath, _ := config.Path()
	configFile := altsrc.StringSourcer(configPath)

	return &cli.Command{
		Name:  "paddi",
		Usage: "Paddi from the command line",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "output raw API JSON", Destination: &opts.JSON},
			&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "only output essential results", Destination: &opts.Quiet},
			&cli.StringFlag{Name: "project", Usage: "override the current project context", Destination: &opts.Project, Sources: cli.NewValueSourceChain(cli.EnvVar(config.EnvProject), toml.TOML(config.KeyProjectID, configFile))},
			&cli.StringFlag{Name: "api-base", Usage: "override the API base URL", Value: config.DefaultAPIBase, Destination: &opts.APIBase, Sources: cli.NewValueSourceChain(cli.EnvVar(config.EnvAPIBase), toml.TOML(config.KeyAPIBase, configFile))},
			&cli.StringFlag{Name: "workspace", Hidden: true, Destination: &opts.WorkspaceID, Sources: cli.NewValueSourceChain(toml.TOML(config.KeyWorkspaceID, configFile))},
		},
		Commands: []*cli.Command{
			authCommand(),
			workspaceCommand(),
			projectCommand(),
			specCommand(),
			requestCommand(),
			captureCommand(),
			sourceCommand(),
		},
	}
}
