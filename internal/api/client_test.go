package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/paddi-app/paddi/internal/config"
	"github.com/paddi-app/paddi/internal/credentials"
)

func TestClientGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer tok")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]Workspace{{ID: "w1", Name: "Acme"}})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Token: "tok"}
	got, _, err := c.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "w1" {
		t.Errorf("ListWorkspaces() = %+v, want one workspace w1", got)
	}
}

func TestClientGet_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "spec_not_found"})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Token: "tok"}
	_, _, err := c.GetSpec(context.Background(), "missing")
	if err == nil {
		t.Fatal("GetSpec() error = nil, want not-found error")
	}
	apiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("GetSpec() error type = %T, want *Error", err)
	}
	if apiErr.Status != http.StatusNotFound || apiErr.Code != "spec_not_found" {
		t.Errorf("GetSpec() error = %+v, want status 404 code spec_not_found", apiErr)
	}
}

func TestClientDo_RefreshesOnceOn401(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if r.Header.Get("Authorization") != "Bearer fresh" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode([]Workspace{})
	}))
	defer srv.Close()

	refreshCalls := 0
	c := &Client{
		BaseURL: srv.URL,
		Token:   "stale",
		Refresh: func(_ context.Context) (string, error) {
			refreshCalls++
			return "fresh", nil
		},
	}

	if _, _, err := c.ListWorkspaces(context.Background()); err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if attempts != 2 {
		t.Errorf("server saw %d attempts, want 2 (stale then fresh)", attempts)
	}
	if refreshCalls != 1 {
		t.Errorf("Refresh called %d times, want exactly 1", refreshCalls)
	}
}

func TestClientDo_DoesNotLoopForeverWhenRefreshedTokenAlsoRejected(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Client{
		BaseURL: srv.URL,
		Token:   "stale",
		Refresh: func(_ context.Context) (string, error) { return "still-bad", nil },
	}

	_, _, err := c.ListWorkspaces(context.Background())
	if err == nil {
		t.Fatal("ListWorkspaces() error = nil, want 401 error")
	}
	if attempts != 2 {
		t.Errorf("server saw %d attempts, want exactly 2 (no infinite retry)", attempts)
	}
}

func TestClientDo_RefreshErrorReturnsOriginal401(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Client{
		BaseURL: srv.URL,
		Token:   "stale",
		Refresh: func(_ context.Context) (string, error) { return "", errors.New("refresh failed") },
	}

	_, _, err := c.ListWorkspaces(context.Background())
	apiErr, ok := err.(*Error)
	if !ok || apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("ListWorkspaces() error = %v, want *Error with Status 401", err)
	}
	if attempts != 1 {
		t.Errorf("server saw %d attempts, want exactly 1 (no retry when Refresh itself errors)", attempts)
	}
}

func TestClientDo_AuthEndpointsNeverRefresh(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	refreshCalls := 0
	c := &Client{
		BaseURL: srv.URL,
		Token:   "stale",
		Refresh: func(_ context.Context) (string, error) {
			refreshCalls++
			return "fresh", nil
		},
	}

	if _, err := c.DeviceCode(context.Background()); err == nil {
		t.Fatal("DeviceCode() error = nil, want 401 error")
	}
	if attempts != 1 {
		t.Errorf("server saw %d attempts, want exactly 1 (auth endpoints must not retry via Refresh)", attempts)
	}
	if refreshCalls != 0 {
		t.Errorf("Refresh called %d times, want 0", refreshCalls)
	}
}

func TestClientGet_DecodeErrorOnMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	_, _, err := c.ListWorkspaces(context.Background())
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Errorf("ListWorkspaces() error = %v, want it to mention %q", err, "decode response")
	}
}

func TestListProjects_QueryParamOmittedWhenWorkspaceIDEmpty(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_ = json.NewEncoder(w).Encode([]Project{})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	if _, _, err := c.ListProjects(context.Background(), ""); err != nil {
		t.Fatalf("ListProjects(\"\") error = %v", err)
	}
	if gotQuery != "" {
		t.Errorf("query = %q, want empty when workspaceID is \"\"", gotQuery)
	}

	if _, _, err := c.ListProjects(context.Background(), "w1"); err != nil {
		t.Fatalf("ListProjects(\"w1\") error = %v", err)
	}
	if gotQuery != "workspace_id=w1" {
		t.Errorf("query = %q, want %q", gotQuery, "workspace_id=w1")
	}
}

func TestIndexSource_PostsWithNoBody(t *testing.T) {
	var gotMethod string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	if err := c.IndexSource(context.Background(), "src1"); err != nil {
		t.Fatalf("IndexSource() error = %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if len(gotBody) != 0 {
		t.Errorf("body = %q, want empty", gotBody)
	}
}

func TestError_Error(t *testing.T) {
	cases := []struct {
		name string
		err  *Error
		want string
	}{
		{"with code", &Error{Status: 404, Code: "spec_not_found"}, "spec_not_found (HTTP 404)"},
		{"without code", &Error{Status: 500}, "HTTP 500"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParseError(t *testing.T) {
	cases := []struct {
		name     string
		raw      []byte
		wantCode string
	}{
		{"code field", []byte(`{"code":"spec_not_found"}`), "spec_not_found"},
		{"falls back to error field when code is absent", []byte(`{"error":"bad_request"}`), "bad_request"},
		{"code field wins over error field", []byte(`{"code":"a","error":"b"}`), "a"},
		{"non-json body decodes to empty code", []byte("plain text error"), ""},
		{"empty body decodes to empty code", []byte(``), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := parseError(500, tc.raw)
			if err.Status != 500 {
				t.Errorf("parseError().Status = %d, want 500", err.Status)
			}
			if err.Code != tc.wantCode {
				t.Errorf("parseError().Code = %q, want %q", err.Code, tc.wantCode)
			}
		})
	}
}

func TestDeviceToken_ErrorMapping(t *testing.T) {
	cases := []struct {
		name    string
		code    string
		wantErr error
	}{
		{"authorization_pending maps to sentinel", "authorization_pending", ErrAuthorizationPending},
		{"slow_down maps to sentinel", "slow_down", ErrSlowDown},
		{"expired_token maps to sentinel", "expired_token", ErrExpiredToken},
		{"code matching is case-insensitive", "AUTHORIZATION_PENDING", ErrAuthorizationPending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]string{"code": tc.code})
			}))
			defer srv.Close()

			c := &Client{BaseURL: srv.URL}
			_, err := c.DeviceToken(context.Background(), "dc1")
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("DeviceToken() error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestClientRefreshToken_Success(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/refresh-token" {
			t.Errorf("path = %q, want /auth/refresh-token", r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(Tokens{AccessToken: "at1", RefreshToken: "rt2"})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	got, err := c.RefreshToken(context.Background(), "rt1")
	if err != nil {
		t.Fatalf("RefreshToken() error = %v", err)
	}
	if got.AccessToken != "at1" || got.RefreshToken != "rt2" {
		t.Errorf("RefreshToken() = %+v, want AccessToken=at1 RefreshToken=rt2", got)
	}
	if gotBody["refresh_token"] != "rt1" {
		t.Errorf("body.refresh_token = %q, want %q", gotBody["refresh_token"], "rt1")
	}
}

func TestClientRefreshToken_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	if _, err := c.RefreshToken(context.Background(), "bad"); err == nil {
		t.Error("RefreshToken() error = nil, want error on 401")
	}
}

func TestDeviceToken_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(Tokens{AccessToken: "at1", RefreshToken: "rt1"})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	got, err := c.DeviceToken(context.Background(), "dc1")
	if err != nil {
		t.Fatalf("DeviceToken() error = %v", err)
	}
	if got.AccessToken != "at1" {
		t.Errorf("DeviceToken().AccessToken = %q, want %q", got.AccessToken, "at1")
	}
}

func TestCurrentUser_PropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	if _, _, err := c.CurrentUser(context.Background()); err == nil {
		t.Error("CurrentUser() error = nil, want the server's 401 to propagate")
	}
}

func TestLockSpec_PropagatesError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	if _, _, err := c.LockSpec(context.Background(), "s1"); err == nil {
		t.Error("LockSpec() error = nil, want the server's 500 to propagate")
	}
}

func TestWorkspaceMemberRole_String(t *testing.T) {
	cases := []struct {
		role WorkspaceMemberRole
		want string
	}{
		{WorkspaceMemberRoleMember, "Member"},
		{WorkspaceMemberRoleAdmin, "Admin"},
		{WorkspaceMemberRoleOwner, "Owner"},
		{WorkspaceMemberRole(99), "99"},
	}
	for _, tc := range cases {
		if got := tc.role.String(); got != tc.want {
			t.Errorf("WorkspaceMemberRole(%d).String() = %q, want %q", tc.role, got, tc.want)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestClientHTTPClient_PrefersConfiguredClientOverDefault(t *testing.T) {
	custom := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader("[]")),
			Header:     make(http.Header),
		}, nil
	})}
	c := &Client{BaseURL: "http://example.invalid", HTTPClient: custom}
	if _, _, err := c.ListWorkspaces(context.Background()); err != nil {
		t.Fatalf("ListWorkspaces() error = %v, want the configured HTTPClient (bypassing the network) to be used", err)
	}
}

type errReadCloser struct{}

func (errReadCloser) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (errReadCloser) Close() error             { return nil }

func TestClientDo_BodyReadErrorPropagates(t *testing.T) {
	c := &Client{
		BaseURL: "http://example.invalid",
		HTTPClient: &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: errReadCloser{}, Header: make(http.Header)}, nil
		})},
	}
	_, _, err := c.ListWorkspaces(context.Background())
	if err == nil || !strings.Contains(err.Error(), "read failed") {
		t.Errorf("ListWorkspaces() error = %v, want the body-read error to propagate", err)
	}
}

func TestClientDo_NetworkErrorIsNotWrappedAsAPIError(t *testing.T) {
	c := &Client{BaseURL: "http://127.0.0.1:0"}
	_, _, err := c.ListWorkspaces(context.Background())
	if err == nil {
		t.Fatal("ListWorkspaces() error = nil, want a network error for an unreachable server")
	}
	if _, ok := err.(*Error); ok {
		t.Errorf("ListWorkspaces() error = %v (type %T), want a raw network error, not *Error", err, err)
	}
}

func TestClientDo_InvalidMethodProducesRequestError(t *testing.T) {
	c := &Client{BaseURL: "http://example.invalid"}
	if _, err := c.do(context.Background(), "BAD METHOD", "/x", nil, nil, nil, false); err == nil {
		t.Error("do() with an invalid HTTP method: error = nil, want error")
	}
}

func TestClientDo_MarshalErrorOnUnsupportedBody(t *testing.T) {
	c := &Client{BaseURL: "http://example.invalid"}
	if _, err := c.do(context.Background(), http.MethodPost, "/x", nil, make(chan int), nil, false); err == nil {
		t.Error("do() with an unmarshalable body: error = nil, want error")
	}
}

func TestDeviceToken_UnknownCodePassesThroughAsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"code": "access_denied"})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	_, err := c.DeviceToken(context.Background(), "dc1")
	apiErr, ok := err.(*Error)
	if !ok || apiErr.Code != "access_denied" {
		t.Fatalf("DeviceToken() error = %v, want *Error with code access_denied", err)
	}
	if errors.Is(err, ErrAuthorizationPending) || errors.Is(err, ErrSlowDown) || errors.Is(err, ErrExpiredToken) {
		t.Errorf("DeviceToken() error = %v, want it not to match any device-flow sentinel", err)
	}
}

func TestNewClient_UsesEnvTokenAndSkipsRefresh(t *testing.T) {
	t.Setenv(config.EnvToken, "env-tok")
	c, err := NewClient("http://example.invalid")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if c.Token != "env-tok" {
		t.Errorf("Token = %q, want %q", c.Token, "env-tok")
	}
	if c.Refresh != nil {
		t.Error("Refresh = non-nil, want nil when the token comes from PADDI_TOKEN")
	}
}

func TestNewClient_PropagatesNotLoggedInError(t *testing.T) {
	keyring.MockInit()
	t.Setenv(config.EnvToken, "")
	if _, err := NewClient("http://example.invalid"); !errors.Is(err, credentials.ErrNotLoggedIn) {
		t.Errorf("NewClient() error = %v, want credentials.ErrNotLoggedIn", err)
	}
}

func TestNewClient_RefreshClosureEndToEnd(t *testing.T) {
	keyring.MockInit()
	t.Setenv(config.EnvToken, "")
	if err := credentials.Store("stale-access", "refresh-1"); err != nil {
		t.Fatalf("credentials.Store() error = %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/auth/refresh-token" {
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["refresh_token"] != "refresh-1" {
				t.Errorf("refresh body.refresh_token = %q, want %q", body["refresh_token"], "refresh-1")
			}
			_ = json.NewEncoder(w).Encode(Tokens{AccessToken: "fresh-access", RefreshToken: "fresh-refresh"})
			return
		}
		if r.Header.Get("Authorization") != "Bearer fresh-access" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode([]Workspace{})
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	if c.Token != "stale-access" {
		t.Fatalf("Token = %q, want %q (loaded from the keyring)", c.Token, "stale-access")
	}
	if c.Refresh == nil {
		t.Fatal("Refresh = nil, want a refresh closure when the token comes from the keyring")
	}

	if _, _, err := c.ListWorkspaces(context.Background()); err != nil {
		t.Fatalf("ListWorkspaces() error = %v, want the 401 to trigger a transparent refresh", err)
	}
	if c.Token != "fresh-access" {
		t.Errorf("Token after refresh = %q, want %q", c.Token, "fresh-access")
	}
	access, err := credentials.AccessToken()
	if err != nil || access != "fresh-access" {
		t.Errorf("credentials.AccessToken() after refresh = (%q, %v), want (\"fresh-access\", nil)", access, err)
	}
}
