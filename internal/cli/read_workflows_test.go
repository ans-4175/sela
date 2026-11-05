//go:build darwin || linux

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/auth"
	"github.com/ans-4175/sela/internal/localstore"
	"golang.org/x/oauth2/google"
)

func TestWorkflowInvalidInputsBeforeAccountCredentialAndState(t *testing.T) {
	a, _, store := fixture(t)
	ctx := context.Background()
	for _, args := range [][]string{
		{"mail", "thread", "--account", "unknown", "--id", "../bad"},
		{"mail", "thread", "--account", "unknown", "--id", "thread1", "--limit", "21"},
		{"mail", "thread", "--account", "unknown", "--id", "thread1", "--mark-read"},
		{"brief", "--account", "unknown", "--file", "/nonexistent/synthetic.json"},
		{"calendar", "availability", "--account", "unknown", "--file", "/nonexistent/synthetic.json"},
	} {
		if err := a.Run(ctx, args); err == nil || strings.Contains(err.Error(), "account alias") {
			t.Fatal(args, err)
		}
	}
	if store.calls != 0 {
		t.Fatal("credential access")
	}
	if _, err := os.Stat(filepath.Dir(a.RegistryPath)); !os.IsNotExist(err) {
		t.Fatal("invalid reads created state")
	}
}
func TestAvailabilityOptInRequiredBeforeCredentialsLegacyConnectionCompatible(t *testing.T) {
	a, _, store := fixture(t)
	ctx := context.Background()
	acct := accounts.Account{Name: "personal", Email: "person@example.test"}
	if err := accounts.Add(ctx, a.RegistryPath, acct); err != nil {
		t.Fatal(err)
	}
	cfg := json.RawMessage(`{"installed":{"client_id":"synthetic.apps.googleusercontent.com","client_secret":"SYNTHETIC_SECRET","auth_uri":"` + google.Endpoint.AuthURL + `","token_uri":"` + google.Endpoint.TokenURL + `","redirect_uris":["http://localhost"]}}`)
	path := filepath.Join(filepath.Dir(a.RegistryPath), "connections", "personal.json")
	if err := localstore.Write(path, Connection{Ready: true, Client: cfg}); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(filepath.Dir(a.RegistryPath), "availability.json")
	os.WriteFile(file, []byte(`{"calendars":["primary"],"start":"2026-12-01T09:00:00+07:00","end":"2026-12-01T17:00:00+07:00","time_zone":"Asia/Jakarta","duration_minutes":30}`), 0600)
	if err := a.Run(ctx, []string{"calendar", "availability", "--account", "personal", "--file", file}); err == nil || !strings.Contains(err.Error(), "--calendar-freebusy") {
		t.Fatal(err)
	}
	if store.calls != 0 {
		t.Fatal("missing capability opened keyring")
	}
	conn, c, err := a.connection(acct)
	if err != nil || conn.CalendarFreeBusy || strings.Contains(strings.Join(c.Scopes, " "), auth.CalendarFreeBusyScope) {
		t.Fatal("legacy scope changed", err)
	}
	conn.CalendarFreeBusy = true
	if err := localstore.Write(path, conn); err != nil {
		t.Fatal(err)
	}
	_, c, err = a.connection(acct)
	if err != nil || !strings.Contains(strings.Join(c.Scopes, " "), auth.CalendarFreeBusyScope) {
		t.Fatal("opt-in not configured", err)
	}
	if err := a.Run(ctx, []string{"auth", "login", "--account", "personal", "--calendar-freebusy"}); err == nil || !strings.Contains(err.Error(), "--profile") {
		t.Fatal("flag not parsed", err)
	}
}
