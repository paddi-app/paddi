package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
	"github.com/paddi-app/paddi/internal/output"
)

func captureCommand() *cli.Command {
	return &cli.Command{
		Name:   "capture",
		Usage:  "Feed raw feedback into Paddi",
		Before: requireProject,
		Commands: []*cli.Command{
			{Name: "list", Usage: "List captures in the current project", Action: runCaptureList},
			{
				Name:      "create",
				Usage:     "Create a capture from a message, file, or stdin",
				ArgsUsage: "[-]",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "message", Aliases: []string{"m"}, Usage: "feedback text"},
					&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Usage: "read feedback from a file"},
				},
				Action: runCaptureCreate,
			},
		},
	}
}

func runCaptureList(ctx context.Context, _ *cli.Command) error {
	client, err := api.NewClient(opts.APIBase)
	if err != nil {
		return err
	}
	captures, raw, err := client.ListCaptures(ctx, opts.Project)
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	if opts.Quiet {
		for _, c := range captures {
			fmt.Println(c.ID)
		}
		return nil
	}
	rows := make([][]string, 0, len(captures))
	for _, c := range captures {
		name := c.Name
		if name == "" {
			name = c.Description
		}
		rows = append(rows, []string{c.ID, output.Truncate(name, 50), c.Status, c.CreatedAt.Local().Format("2006-01-02 15:04")})
	}
	return output.Table(os.Stdout, []string{"ID", "NAME", "STATUS", "CREATED"}, rows)
}

func runCaptureCreate(ctx context.Context, cmd *cli.Command) error {
	description, err := captureDescription(cmd)
	if err != nil {
		return err
	}
	projectID, err := cmdutil.RequireProject(opts.Project)
	if err != nil {
		return err
	}
	client, err := api.NewClient(opts.APIBase)
	if err != nil {
		return err
	}
	capture, raw, err := client.CreateCapture(ctx, api.CaptureInput{
		ProjectID:   projectID,
		Description: description,
	})
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	if opts.Quiet {
		fmt.Println(capture.ID)
	} else {
		fmt.Printf("Capture %s created.\n", capture.ID)
	}
	return nil
}

func captureDescription(cmd *cli.Command) (string, error) {
	switch {
	case cmd.String("message") != "":
		return cmd.String("message"), nil
	case cmd.String("file") != "":
		data, err := cmdutil.ReadFileOrStdin(cmd.String("file"))
		if err != nil {
			return "", err
		}
		return validDescription(string(data))
	case cmd.Args().First() == "-":
		data, err := cmdutil.ReadFileOrStdin("-")
		if err != nil {
			return "", err
		}
		return validDescription(string(data))
	default:
		return "", errors.New("provide feedback via -m <message>, -f <file>, or '-' for stdin")
	}
}

func validDescription(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New("feedback is empty")
	}
	return s, nil
}
