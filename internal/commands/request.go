package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
	"github.com/paddi-app/paddi/internal/output"
)

func requestCommand() *cli.Command {
	return &cli.Command{
		Name:   "request",
		Usage:  "Work with feedback requests",
		Before: requireProject,
		Commands: []*cli.Command{
			{Name: "list", Usage: "List requests in the current project, sorted by RIGE score", Action: runRequestList},
			{Name: "view", Usage: "Show a request's analysis, score and solution paths", ArgsUsage: "<request-id>", Action: runRequestView},
			{
				Name:      "regenerate",
				Usage:     "Regenerate a request's solution paths",
				ArgsUsage: "<request-id>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "expectation", Aliases: []string{"e"}, Usage: "expectation guiding the regeneration"},
				},
				Action: runRequestRegenerate,
			},
			{
				Name:      "draft",
				Usage:     "Answer solution paths and trigger spec generation",
				ArgsUsage: "<request-id>",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "file", Aliases: []string{"f"}, Required: true, Usage: "answers JSON file (use '-' for stdin)"},
				},
				Action: runRequestDraft,
			},
		},
	}
}

func runRequestList(ctx context.Context, _ *cli.Command) error {
	client, err := api.NewClient(opts.APIBase)
	if err != nil {
		return err
	}
	requests, raw, err := client.ListRequests(ctx, opts.Project)
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	sort.SliceStable(requests, func(i, j int) bool { return requests[i].Score > requests[j].Score })
	if opts.Quiet {
		for _, r := range requests {
			fmt.Println(r.ID)
		}
		return nil
	}
	rows := make([][]string, 0, len(requests))
	for _, r := range requests {
		rows = append(rows, []string{r.ID, output.Truncate(r.Name, 50), r.Type, r.Status, fmt.Sprintf("%.1f", r.Score)})
	}
	return output.Table(os.Stdout, []string{"ID", "NAME", "TYPE", "STATUS", "SCORE"}, rows)
}

func runRequestView(ctx context.Context, cmd *cli.Command) error {
	id, err := cmdutil.SingleArg(cmd, "paddi request view <request-id>")
	if err != nil {
		return err
	}
	client, err := api.NewClient(opts.APIBase)
	if err != nil {
		return err
	}
	req, raw, err := client.GetRequest(ctx, id)
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	output.Request(os.Stdout, req)
	return nil
}

func runRequestRegenerate(ctx context.Context, cmd *cli.Command) error {
	id, err := cmdutil.SingleArg(cmd, "paddi request regenerate <request-id> [-e <expectation>]")
	if err != nil {
		return err
	}
	client, err := api.NewClient(opts.APIBase)
	if err != nil {
		return err
	}
	req, raw, err := client.RegenerateRequest(ctx, id, cmd.String("expectation"))
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	if !opts.Quiet {
		fmt.Printf("Regenerating solution paths for %q (status: %s)\n", req.Name, req.Status)
	}
	return nil
}

func runRequestDraft(ctx context.Context, cmd *cli.Command) error {
	id, err := cmdutil.SingleArg(cmd, "paddi request draft <request-id> -f <answers.json>")
	if err != nil {
		return err
	}
	answers, err := readAnswers(cmd.String("file"))
	if err != nil {
		return err
	}
	client, err := api.NewClient(opts.APIBase)
	if err != nil {
		return err
	}
	req, raw, err := client.DraftRequest(ctx, id, answers)
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	if !opts.Quiet {
		fmt.Printf("Drafting spec for %q (status: %s)\n", req.Name, req.Status)
	}
	return nil
}

// readAnswers accepts either a bare answers array or a {"answers": [...]} object.
func readAnswers(path string) ([]api.Answer, error) {
	data, err := cmdutil.ReadFileOrStdin(path)
	if err != nil {
		return nil, err
	}

	data = bytes.TrimSpace(data)
	var answers []api.Answer
	if len(data) > 0 && data[0] == '[' {
		err = json.Unmarshal(data, &answers)
	} else {
		var body struct {
			Answers []api.Answer `json:"answers"`
		}
		err = json.Unmarshal(data, &body)
		answers = body.Answers
	}
	if err != nil {
		return nil, fmt.Errorf("invalid answers JSON: %w", err)
	}
	if len(answers) == 0 {
		return nil, errors.New("answers must contain at least one entry")
	}
	return answers, nil
}
