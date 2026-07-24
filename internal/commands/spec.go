package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/cmdutil"
	"github.com/paddi-app/paddi/internal/output"
)

func specCommand() *cli.Command {
	return &cli.Command{
		Name:   "spec",
		Usage:  "Work with specs",
		Before: requireProject,
		Commands: []*cli.Command{
			{Name: "list", Usage: "List specs in the current project", Action: runSpecList},
			{Name: "info", Usage: "Print a spec's metadata (without its markdown content)", ArgsUsage: "<spec-id>", Action: runSpecInfo},
			{Name: "view", Usage: "Print a spec's markdown content", ArgsUsage: "<spec-id>", Action: runSpecView},
			{
				Name:      "download",
				Usage:     "Write a spec's markdown content to a local file",
				ArgsUsage: "<spec-id>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "output", Aliases: []string{"o"}, Usage: "output file path (default: <title>.md)"},
				},
				Action: runSpecDownload,
			},
			{Name: "lock", Usage: "Lock a spec to prevent further edits", ArgsUsage: "<spec-id>", Action: runSpecLock},
		},
	}
}

func runSpecList(ctx context.Context, _ *cli.Command) error {
	client, err := newClient()
	if err != nil {
		return err
	}
	specs, raw, err := client.ListSpecs(ctx, opts.Project)
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	sort.SliceStable(specs, func(i, j int) bool { return specs[i].CreatedAt.After(specs[j].CreatedAt) })
	if opts.Quiet {
		for _, s := range specs {
			fmt.Println(s.ID)
		}
		return nil
	}
	rows := make([][]string, 0, len(specs))
	for _, s := range specs {
		requestType := ""
		if s.Request != nil {
			requestType = s.Request.Type
		}
		locked := "no"
		if s.Locked {
			locked = "yes"
		}
		rows = append(rows, []string{s.ID, output.Truncate(s.Title, 50), requestType, locked, s.CreatedAt.Local().Format("2006-01-02")})
	}
	return output.Table(os.Stdout, []string{"ID", "TITLE", "REQUEST TYPE", "LOCKED", "CREATED"}, rows)
}

func runSpecInfo(ctx context.Context, cmd *cli.Command) error {
	id, err := cmdutil.SingleArg(cmd, "paddi spec info <spec-id>")
	if err != nil {
		return err
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	spec, _, err := client.GetSpec(ctx, id)
	if err != nil {
		return err
	}
	if opts.JSON {
		spec.Content = ""
		b, err := json.Marshal(spec)
		if err != nil {
			return err
		}
		return output.JSON(os.Stdout, b)
	}
	locked := "no"
	if spec.Locked {
		locked = "yes"
	}
	fmt.Printf("%s (%s)\n", spec.Title, spec.ID)
	fmt.Printf("Request ID: %s\n", spec.RequestID)
	fmt.Printf("Locked: %s\n", locked)
	fmt.Printf("Created: %s\n", spec.CreatedAt.Local().Format("2006-01-02 15:04"))
	return nil
}

func runSpecView(ctx context.Context, cmd *cli.Command) error {
	id, err := cmdutil.SingleArg(cmd, "paddi spec view <spec-id>")
	if err != nil {
		return err
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	spec, raw, err := client.GetSpec(ctx, id)
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	fmt.Printf("# %s\n", spec.Title)
	fmt.Print(spec.Content)
	if !strings.HasSuffix(spec.Content, "\n") {
		fmt.Println()
	}
	return nil
}

func runSpecDownload(ctx context.Context, cmd *cli.Command) error {
	id, err := cmdutil.SingleArg(cmd, "paddi spec download <spec-id> [-o <path>]")
	if err != nil {
		return err
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	spec, _, err := client.GetSpec(ctx, id)
	if err != nil {
		return err
	}
	path := cmd.String("output")
	if path == "" {
		path = output.SpecFilename(spec)
	}
	if err := os.WriteFile(path, []byte(spec.Content), 0o644); err != nil {
		return err
	}
	if opts.Quiet {
		fmt.Println(path)
	} else {
		fmt.Printf("Wrote spec %q to %s\n", spec.Title, path)
	}
	return nil
}

func runSpecLock(ctx context.Context, cmd *cli.Command) error {
	id, err := cmdutil.SingleArg(cmd, "paddi spec lock <spec-id>")
	if err != nil {
		return err
	}
	client, err := newClient()
	if err != nil {
		return err
	}
	spec, raw, err := client.LockSpec(ctx, id)
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	if !opts.Quiet {
		fmt.Printf("Spec %q locked.\n", spec.Title)
	}
	return nil
}
