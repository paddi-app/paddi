package commands

import (
	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/cmdutil"
)

var opts cmdutil.Options

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
	return &cli.Command{
		Name:  "paddi",
		Usage: "Paddi from the command line",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "json", Usage: "output raw API JSON", Destination: &opts.JSON},
			&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "only output essential results", Destination: &opts.Quiet},
			&cli.StringFlag{Name: "project", Usage: "override the current project context", Destination: &opts.Project},
			&cli.StringFlag{Name: "api-base", Usage: "override the API base URL", Destination: &opts.APIBase},
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
