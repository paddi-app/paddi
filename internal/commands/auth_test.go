package commands

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/paddi-app/paddi/internal/api"
	"github.com/paddi-app/paddi/internal/cmdutil"
	"github.com/paddi-app/paddi/internal/config"
	"github.com/paddi-app/paddi/internal/credentials"
)

func TestRunAuthLogin_DeviceCodeErrorPropagates(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if err := runAuthLogin(context.Background(), nil); err == nil {
		t.Error("runAuthLogin() error = nil, want the device-code request's error to propagate")
	}
}

func TestRunAuthLogin_ExpiredTokenStopsPolling(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/device/code":
			_ = json.NewEncoder(w).Encode(api.DeviceCode{DeviceCode: "dc1", UserCode: "ABCD", VerificationURI: "https://example.com", Interval: 1, ExpiresIn: 60})
		case "/auth/device/token":
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"code": "expired_token"})
		}
	})
	err := runAuthLogin(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("runAuthLogin() error = %v, want a device-code-expired error", err)
	}
}

func TestRunAuthLogin_PendingThenSuccessStoresCredentials(t *testing.T) {
	keyring.MockInit()
	polls := 0
	withAPIBase(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/device/code":
			_ = json.NewEncoder(w).Encode(api.DeviceCode{DeviceCode: "dc1", UserCode: "ABCD", VerificationURI: "https://example.com", Interval: 1, ExpiresIn: 60})
		case "/auth/device/token":
			polls++
			if polls == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": "authorization_pending"})
				return
			}
			_ = json.NewEncoder(w).Encode(api.Tokens{AccessToken: "at1", RefreshToken: "rt1"})
		}
	})
	// runAuthLogin stores via the keyring directly; PADDI_TOKEN (set by
	// withAPIBase for client construction elsewhere) must not shadow it here.
	t.Setenv(config.EnvToken, "")
	out := captureStdout(t, func() {
		if err := runAuthLogin(context.Background(), nil); err != nil {
			t.Fatalf("runAuthLogin() error = %v", err)
		}
	})
	if !strings.Contains(out, "Logged in.") {
		t.Errorf("stdout = %q, want it to end with \"Logged in.\"", out)
	}
	if polls != 2 {
		t.Errorf("server saw %d token polls, want 2 (pending then success)", polls)
	}
	access, err := credentials.AccessToken()
	if err != nil || access != "at1" {
		t.Errorf("credentials.AccessToken() = (%q, %v), want (\"at1\", nil)", access, err)
	}
}

func TestRunAuthLogout_NoStoredCredentialsSkipsRevokeCall(t *testing.T) {
	keyring.MockInit()
	t.Setenv(config.EnvToken, "")
	withConfigFile(t)

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	prevBase := opts.APIBase
	opts.APIBase = srv.URL
	t.Cleanup(func() { opts.APIBase = prevBase })

	out := captureStdout(t, func() {
		if err := runAuthLogout(context.Background(), nil); err != nil {
			t.Fatalf("runAuthLogout() error = %v", err)
		}
	})
	if called {
		t.Error("runAuthLogout() called /auth/logout despite no stored refresh token")
	}
	if !strings.Contains(out, "Logged out.") {
		t.Errorf("stdout = %q, want it to mention \"Logged out.\"", out)
	}
}

func TestRunAuthLogout_WithStoredCredentialsRevokesAndClears(t *testing.T) {
	keyring.MockInit()
	withConfigFile(t)
	if err := credentials.Store("at1", "rt1"); err != nil {
		t.Fatalf("credentials.Store() error = %v", err)
	}

	var gotBody map[string]string
	withAPIBase(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	})
	// credentials.Clear()/AccessToken() below must see the keyring, not PADDI_TOKEN.
	t.Setenv(config.EnvToken, "")

	captureStdout(t, func() {
		if err := runAuthLogout(context.Background(), nil); err != nil {
			t.Fatalf("runAuthLogout() error = %v", err)
		}
	})
	if gotBody["refresh_token"] != "rt1" {
		t.Errorf("logout request body = %+v, want refresh_token rt1", gotBody)
	}
	if _, err := credentials.AccessToken(); !errors.Is(err, credentials.ErrNotLoggedIn) {
		t.Errorf("AccessToken() after logout error = %v, want ErrNotLoggedIn", err)
	}
}

func TestRunAuthLogout_RevokeFailureStillClearsLocally(t *testing.T) {
	keyring.MockInit()
	withConfigFile(t)
	if err := credentials.Store("at1", "rt1"); err != nil {
		t.Fatalf("credentials.Store() error = %v", err)
	}
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	t.Setenv(config.EnvToken, "")

	stderr := captureStderr(t, func() {
		captureStdout(t, func() {
			if err := runAuthLogout(context.Background(), nil); err != nil {
				t.Fatalf("runAuthLogout() error = %v, want nil even when the server-side revoke fails", err)
			}
		})
	})
	if !strings.Contains(stderr, "warning") {
		t.Errorf("stderr = %q, want a revoke-failure warning", stderr)
	}
	if _, err := credentials.AccessToken(); !errors.Is(err, credentials.ErrNotLoggedIn) {
		t.Errorf("AccessToken() after logout error = %v, want ErrNotLoggedIn (local state must still clear)", err)
	}
}

func TestRunAuthStatus_Success(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/user":
			_ = json.NewEncoder(w).Encode(api.User{ID: "u1", Name: "Ada", Email: "ada@example.com"})
		case "/workspace":
			_ = json.NewEncoder(w).Encode([]api.Workspace{{ID: "w1", Name: "Acme", Projects: []api.Project{{ID: "p1", Name: "Website"}}}})
		}
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false; o.WorkspaceID = "w1"; o.Project = "p1" })
	out := captureStdout(t, func() {
		if err := runAuthStatus(context.Background(), nil); err != nil {
			t.Fatalf("runAuthStatus() error = %v", err)
		}
	})
	want := "Logged in as Ada (ada@example.com)\nWorkspace: Acme\nProject:   Website\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestRunAuthStatus_NoContextShowsNotSetPlaceholders(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.User{ID: "u1", Name: "Ada", Email: "ada@example.com"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = false; o.WorkspaceID = ""; o.Project = "" })
	out := captureStdout(t, func() {
		if err := runAuthStatus(context.Background(), nil); err != nil {
			t.Fatalf("runAuthStatus() error = %v", err)
		}
	})
	if !strings.Contains(out, "Workspace: (not set)") || !strings.Contains(out, "Project:   (not set)") {
		t.Errorf("stdout = %q, want (not set) placeholders for an empty context", out)
	}
}

func TestRunAuthStatus_JSON(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(api.User{ID: "u1", Name: "Ada", Email: "ada@example.com"})
	})
	withOpts(t, func(o *cmdutil.Options) { o.JSON = true })
	out := captureStdout(t, func() {
		if err := runAuthStatus(context.Background(), nil); err != nil {
			t.Fatalf("runAuthStatus() error = %v", err)
		}
	})
	if !strings.Contains(out, `"id":"u1"`) {
		t.Errorf("stdout = %q, want raw JSON with id u1", out)
	}
}

func TestRunAuthStatus_PropagatesCurrentUserError(t *testing.T) {
	withAPIBase(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	if err := runAuthStatus(context.Background(), nil); err == nil {
		t.Error("runAuthStatus() error = nil, want the server's 401 to propagate")
	}
}
