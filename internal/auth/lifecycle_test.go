//go:build darwin || linux

package auth

import (
	"context"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSyntheticLifecycleValidExpiryRotationAndRevocation(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		token                *oauth2.Token
		response             string
		status, calls, saves int
		wantErr              bool
	}{
		{"valid_no_refresh", &oauth2.Token{AccessToken: "SYNTHETIC_VALID", RefreshToken: "SYNTHETIC_REFRESH", Expiry: time.Now().Add(time.Hour)}, "", 200, 0, 0, false},
		{"expired_rotates_refresh", &oauth2.Token{AccessToken: "SYNTHETIC_OLD", RefreshToken: "SYNTHETIC_REFRESH", Expiry: time.Now().Add(-time.Hour)}, `{"access_token":"SYNTHETIC_NEW","refresh_token":"SYNTHETIC_ROTATED","token_type":"Bearer","expires_in":3600}`, 200, 1, 1, false},
		{"missing_refresh", &oauth2.Token{AccessToken: "SYNTHETIC_OLD", Expiry: time.Now().Add(-time.Hour)}, "", 200, 0, 0, true},
		{"revoked", &oauth2.Token{AccessToken: "SYNTHETIC_OLD", RefreshToken: "SYNTHETIC_REFRESH", Expiry: time.Now().Add(-time.Hour)}, `{"error":"invalid_grant","error_description":"SYNTHETIC_PRIVATE"}`, 400, 1, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, _ := filepath.EvalSymlinks(t.TempDir())
			os.Chmod(dir, 0700)
			store := &memory{t: tc.token}
			calls := 0
			ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) { calls++; return response(tc.response, tc.status), nil })})
			cfg := &oauth2.Config{ClientID: "synthetic", Endpoint: oauth2.Endpoint{TokenURL: "https://fixture.invalid/token", AuthStyle: oauth2.AuthStyleInParams}}
			src, err := TokenSource(ctx, cfg, store, "fixture", dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			tok, err := src.Token()
			if (err != nil) != tc.wantErr {
				t.Fatal(err)
			}
			if err != nil && (tok != nil || strings.Contains(err.Error(), "SYNTHETIC")) {
				t.Fatal("credential leak")
			}
			if calls != tc.calls || store.saves != tc.saves {
				t.Fatal(calls, store.saves)
			}
			if tc.name == "expired_rotates_refresh" {
				tok2, err := src.Token()
				if err != nil || tok2.RefreshToken != "SYNTHETIC_ROTATED" || calls != 1 {
					t.Fatal("rotation not persisted")
				}
			}
		})
	}
}

func TestCancelledRefreshNeverContactsProvider(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	os.Chmod(dir, 0700)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) { calls++; return response(`{}`, 200), nil })})
	src, err := TokenSource(ctx, &oauth2.Config{ClientID: "synthetic"}, &memory{t: &oauth2.Token{RefreshToken: "SYNTHETIC_REFRESH"}}, "fixture", dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = src.Token(); err == nil || calls != 0 {
		t.Fatal("cancelled request sent", err, calls)
	}
}
