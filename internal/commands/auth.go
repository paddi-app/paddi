package commands

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/urfave/cli/v3"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
	"github.com/paddi-app/paddi/internal/config"
	"github.com/paddi-app/paddi/internal/credentials"
	"github.com/paddi-app/paddi/internal/output"
	"github.com/paddi-app/paddi/pkg/browser"
)

func authCommand() *cli.Command {
	return &cli.Command{
		Name:  "auth",
		Usage: "Authenticate with Paddi",
		Commands: []*cli.Command{
			{Name: "login", Usage: "Log in via the browser (device flow)", Action: runAuthLogin},
			{Name: "logout", Usage: "Revoke the session and clear local credentials", Action: runAuthLogout},
			{Name: "status", Usage: "Show the logged-in user and current context", Action: runAuthStatus},
		},
	}
}

func runAuthLogin(ctx context.Context, _ *cli.Command) error {
	client := &api.Client{BaseURL: opts.APIBase}

	code, err := client.DeviceCode(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("First copy your one-time code: %s\n", code.UserCode)
	if stdinIsTTY() {
		fmt.Printf("Press Enter to open %s in your browser, or open it yourself.\n", code.VerificationURI)
		go openBrowserOnEnter(ctx, code.VerificationURI)
	} else {
		fmt.Printf("Open %s and enter the code.\n", code.VerificationURI)
	}
	fmt.Println("Waiting for approval...")

	interval := time.Duration(code.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	expiresIn := time.Duration(code.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = 15 * time.Minute
	}
	deadline := time.Now().Add(expiresIn)

	for {
		if time.Now().After(deadline) {
			return errors.New("login timed out: device code expired")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}

		tokens, err := client.DeviceToken(ctx, code.DeviceCode)
		switch {
		case errors.Is(err, api.ErrAuthorizationPending):
			continue
		case errors.Is(err, api.ErrSlowDown):
			interval += 5 * time.Second
			continue
		case errors.Is(err, api.ErrExpiredToken):
			return errors.New("login timed out: device code expired")
		case err != nil:
			return err
		}

		if err := credentials.Store(tokens.AccessToken, tokens.RefreshToken); err != nil {
			return err
		}
		fmt.Println("Logged in.")
		return nil
	}
}

func stdinIsTTY() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func openBrowserOnEnter(ctx context.Context, url string) {
	if err := waitForEnter(ctx); err != nil {
		return
	}
	if err := browser.OpenURL(url); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open browser: %v\nOpen %s manually.\n", err, url)
	}
}

func waitForEnter(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		close(done)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

func runAuthLogout(ctx context.Context, _ *cli.Command) error {
	if rt, err := credentials.RefreshToken(); err == nil {
		token, _ := credentials.AccessToken()
		client := &api.Client{BaseURL: opts.APIBase, Token: token}
		if err := client.Logout(ctx, rt); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to revoke session: %v\n", err)
		}
	}
	if err := credentials.Clear(); err != nil {
		return err
	}
	if err := config.Clear(); err != nil {
		return err
	}
	if !opts.Quiet {
		fmt.Println("Logged out.")
	}
	return nil
}

func runAuthStatus(ctx context.Context, _ *cli.Command) error {
	client, err := api.NewClient(opts.APIBase)
	if err != nil {
		return err
	}
	user, raw, err := client.CurrentUser(ctx)
	if err != nil {
		return err
	}
	if opts.JSON {
		return output.JSON(os.Stdout, raw)
	}
	workspace, project := cmdutil.ResolveContextNames(ctx, client, opts.WorkspaceID, opts.Project)
	fmt.Printf("Logged in as %s (%s)\n", user.Name, user.Email)
	fmt.Printf("Workspace: %s\n", output.OrNone(workspace))
	fmt.Printf("Project:   %s\n", output.OrNone(project))
	return nil
}
