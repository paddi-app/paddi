package credentials

import (
	"errors"
	"os"
	"testing"

	"github.com/zalando/go-keyring"

	"github.com/paddi-app/paddi/internal/config"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

func TestAccessToken_PrefersEnvOverKeyring(t *testing.T) {
	t.Setenv(config.EnvToken, "env-token")
	got, err := AccessToken()
	if err != nil {
		t.Fatalf("AccessToken() error = %v", err)
	}
	if got != "env-token" {
		t.Errorf("AccessToken() = %q, want %q", got, "env-token")
	}
	if !FromEnv() {
		t.Error("FromEnv() = false, want true when PADDI_TOKEN is set")
	}
}

func TestAccessToken_NotLoggedIn(t *testing.T) {
	t.Setenv(config.EnvToken, "")
	if err := Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	_, err := AccessToken()
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("AccessToken() error = %v, want ErrNotLoggedIn", err)
	}
}

func TestFromEnv_FalseWhenUnset(t *testing.T) {
	t.Setenv(config.EnvToken, "")
	if FromEnv() {
		t.Error("FromEnv() = true, want false when PADDI_TOKEN is unset")
	}
}

func TestRefreshToken_NotLoggedIn(t *testing.T) {
	if err := Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	_, err := RefreshToken()
	if !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("RefreshToken() error = %v, want ErrNotLoggedIn", err)
	}
}

func TestClear_NoopWhenNothingStored(t *testing.T) {
	if err := Clear(); err != nil {
		t.Fatalf("first Clear() error = %v", err)
	}
	if err := Clear(); err != nil {
		t.Errorf("second Clear() on already-empty store: error = %v, want nil", err)
	}
}

func TestClear_HandlesPartiallyStoredKeys(t *testing.T) {
	t.Setenv(config.EnvToken, "")
	if err := Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if err := keyring.Set(service, accessTokenKey, "only-access"); err != nil {
		t.Fatalf("keyring.Set() error = %v", err)
	}
	if err := Clear(); err != nil {
		t.Errorf("Clear() with only access token stored (no refresh token): error = %v, want nil", err)
	}
	if _, err := AccessToken(); !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("AccessToken() after Clear() error = %v, want ErrNotLoggedIn", err)
	}
}

func TestStoreAndRetrieve(t *testing.T) {
	t.Setenv(config.EnvToken, "")
	if err := Store("access-1", "refresh-1"); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	access, err := AccessToken()
	if err != nil || access != "access-1" {
		t.Errorf("AccessToken() = (%q, %v), want (\"access-1\", nil)", access, err)
	}
	refresh, err := RefreshToken()
	if err != nil || refresh != "refresh-1" {
		t.Errorf("RefreshToken() = (%q, %v), want (\"refresh-1\", nil)", refresh, err)
	}

	if err := Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if _, err := AccessToken(); !errors.Is(err, ErrNotLoggedIn) {
		t.Errorf("AccessToken() after Clear() error = %v, want ErrNotLoggedIn", err)
	}
}
