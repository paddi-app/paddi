package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeviceCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/auth/device/code" {
			t.Errorf("request = %s %s, want POST /auth/device/code", r.Method, r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(DeviceCode{UserCode: "ABCD-1234", VerificationURI: "https://example.com/device", Interval: 5, ExpiresIn: 900})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	got, err := c.DeviceCode(context.Background())
	if err != nil {
		t.Fatalf("DeviceCode() error = %v", err)
	}
	if got.UserCode != "ABCD-1234" || got.Interval != 5 {
		t.Errorf("DeviceCode() = %+v", got)
	}
}

func TestLogout(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	if err := c.Logout(context.Background(), "rt-1"); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if gotBody["refresh_token"] != "rt-1" {
		t.Errorf("Logout() body = %+v, want refresh_token=rt-1", gotBody)
	}
}

func TestCurrentUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/user" {
			t.Errorf("path = %s, want /user", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(User{ID: "u1", Name: "Ada", Email: "ada@example.com"})
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL}
	got, _, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("CurrentUser() error = %v", err)
	}
	if got.ID != "u1" || got.Email != "ada@example.com" {
		t.Errorf("CurrentUser() = %+v", got)
	}
}
