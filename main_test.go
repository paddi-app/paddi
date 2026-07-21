package main

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/credentials"
)

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"unauthorized", &api.Error{Status: 401}, exitAuth},
		{"forbidden", &api.Error{Status: 403}, exitAuth},
		{"server error", &api.Error{Status: 500}, exitServer},
		{"API user error", &api.Error{Status: 400}, exitUserError},
		{"not logged in", credentials.ErrNotLoggedIn, exitAuth},
		{"URL error", &url.Error{Op: "Get", URL: "https://example.invalid", Err: errors.New("failed")}, exitServer},
		{"network error", &net.DNSError{Err: "failed", Name: "example.invalid"}, exitServer},
		{"generic error", errors.New("bad input"), exitUserError},
		{"wrapped API error", fmt.Errorf("request failed: %w", &api.Error{Status: 503}), exitServer},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := exitCode(tt.err); got != tt.want {
				t.Errorf("exitCode(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}
