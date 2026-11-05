//go:build darwin || linux

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/localstore"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type diagnosticStore struct {
	loads, saves int
	token        *oauth2.Token
}

func (s *diagnosticStore) Load(string) (*oauth2.Token, error) { s.loads++; return s.token, nil }
func (s *diagnosticStore) Save(string, *oauth2.Token) error   { s.saves++; return nil }

func TestDoctorOfflineDefaultAndCredentialRedaction(t *testing.T) {
	a, out, forbidden := fixture(t)
	ctx := context.Background()
	for _, args := range [][]string{{"doctor"}, {"doctor", "--account", "personal"}} {
		if err := a.Run(ctx, args); err != nil {
			t.Fatal(err)
		}
	}
	if forbidden.calls != 0 {
		t.Fatal("doctor opened keyring")
	}
	if _, err := os.Stat(filepath.Dir(a.RegistryPath)); !os.IsNotExist(err) {
		t.Fatal("doctor created state")
	}
	if a.Run(ctx, []string{"doctor", "--credential-check"}) == nil {
		t.Fatal("implicit account")
	}
	acct := accounts.Account{Name: "personal", Email: "person@example.test"}
	if err := accounts.Add(ctx, a.RegistryPath, acct); err != nil {
		t.Fatal(err)
	}
	cfg := json.RawMessage(`{"installed":{"client_id":"synthetic.apps.googleusercontent.com","client_secret":"SYNTHETIC_SECRET","auth_uri":"` + google.Endpoint.AuthURL + `","token_uri":"` + google.Endpoint.TokenURL + `","redirect_uris":["http://localhost"]}}`)
	if err := localstore.Write(filepath.Join(filepath.Dir(a.RegistryPath), "connections", "personal.json"), Connection{Ready: true, Client: cfg}); err != nil {
		t.Fatal(err)
	}
	store := &diagnosticStore{token: &oauth2.Token{AccessToken: "SYNTHETIC_ACCESS_PRIVATE", RefreshToken: "SYNTHETIC_REFRESH_PRIVATE", Expiry: time.Now().Add(-time.Hour)}}
	a.Credentials = store
	out.Reset()
	if err := a.Run(ctx, []string{"doctor", "--account", "personal"}); err != nil {
		t.Fatal(err)
	}
	if store.loads != 0 {
		t.Fatal("default credential access")
	}
	out.Reset()
	if err := a.Run(ctx, []string{"doctor", "--account", "personal", "--credential-check"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "SYNTHETIC") || !strings.Contains(out.String(), "expired_or_expiring") || store.loads != 1 || store.saves != 0 {
		t.Fatal("unsafe diagnostic", out.String())
	}
	var v map[string]any
	json.Unmarshal(out.Bytes(), &v)
	if v["refresh_attempted"] != false || v["network_used"] != false || v["live_identity_checked"] != false {
		t.Fatal(v)
	}
}
