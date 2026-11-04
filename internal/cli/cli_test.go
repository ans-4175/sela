//go:build darwin || linux

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/credentials"
	"github.com/ans-4175/sela/internal/plans"
	"golang.org/x/oauth2"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type forbiddenStore struct{ calls int }

func (s *forbiddenStore) Load(string) (*oauth2.Token, error) {
	s.calls++
	return nil, credentials.ErrUnavailable
}
func (s *forbiddenStore) Save(string, *oauth2.Token) error {
	s.calls++
	return errors.New("SYNTHETIC_SECRET")
}
func fixture(t *testing.T) (*App, *bytes.Buffer, *forbiddenStore) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	out := &bytes.Buffer{}
	creds := &forbiddenStore{}
	a := &App{Out: out, RegistryPath: filepath.Join(dir, "sela", "accounts.json"), Credentials: creds}
	return a, out, creds
}
func TestDiscoveryIsLocalSelfDescribingAndStrict(t *testing.T) {
	a, out, creds := fixture(t)
	for _, args := range [][]string{{"--help"}, {"version"}, {"auth", "scopes"}, {"--skill"}, {"--skill", "gmail"}, {"--skill", "calendar"}, {"--skill", "workflows"}, {"--skill", "safety"}} {
		out.Reset()
		if e := a.Run(context.Background(), args); e != nil || out.Len() == 0 {
			t.Fatal(args, e)
		}
	}
	for _, args := range [][]string{{"--skill", "../secrets"}, {"--skill", "gmail", "extra"}, {"version", "extra"}, {"mail", "send", "--force"}, {"calendar", "write", "--raw"}} {
		if a.Run(context.Background(), args) == nil {
			t.Fatal("unsupported accepted", args)
		}
	}
	if creds.calls != 0 {
		t.Fatal("discovery touched credentials")
	}
	if _, e := os.Stat(filepath.Dir(a.RegistryPath)); !os.IsNotExist(e) {
		t.Fatal("discovery created state", e)
	}
}
func TestLocalPreviewApprovalAccountAndExtraFlags(t *testing.T) {
	a, out, creds := fixture(t)
	ctx := context.Background()
	for _, acct := range []accounts.Account{{Name: "personal", Email: "person@example.test"}, {Name: "work", Email: "work@example.test"}} {
		if e := accounts.Add(ctx, a.RegistryPath, acct); e != nil {
			t.Fatal(e)
		}
	}
	file := filepath.Join(filepath.Dir(a.RegistryPath), "fixture-input.json")
	os.WriteFile(file, []byte(`{"to":["recipient@example.test"],"subject":"Fixture","body":"exact"}`), 0600)
	if e := a.Run(ctx, []string{"mail", "preview", "--account", "personal", "--action", "send", "--file", file}); e != nil {
		t.Fatal(e)
	}
	var p plans.Plan
	if e := json.Unmarshal(out.Bytes(), &p); e != nil || p.State != "prepared" {
		t.Fatal(e)
	}
	if creds.calls != 0 {
		t.Fatal("preview touched credentials")
	}
	approvals := 0
	a.Confirm = func(got plans.Plan) error {
		approvals++
		if got.Hash != p.Hash {
			t.Fatal("wrong scope")
		}
		return nil
	}
	if a.Run(ctx, []string{"plan", "approve", "--account", "work", "--id", p.Scope.ID}) == nil {
		t.Fatal("cross account approval")
	}
	if approvals != 0 {
		t.Fatal("wrong approval reached human")
	}
	out.Reset()
	if e := a.Run(ctx, []string{"plan", "approve", "--account", "personal", "--id", p.Scope.ID}); e != nil {
		t.Fatal(e)
	}
	if approvals != 1 {
		t.Fatal("approval not called")
	}
	for _, args := range [][]string{{"mail", "search", "--query", "hi"}, {"plan", "execute", "--account", "personal", "--id", p.Scope.ID, "--force"}, {"auth", "login", "--account", "personal"}} {
		if a.Run(ctx, args) == nil {
			t.Fatal("unsafe/missing args accepted")
		}
	}
	if err := a.Run(ctx, []string{"auth", "login", "--account", "personal", "--reauth"}); err == nil || !strings.Contains(err.Error(), "--profile") {
		t.Fatal("re-consent flag did not reach explicit profile validation", err)
	}
	invalidCalendar := filepath.Join(filepath.Dir(a.RegistryPath), "invalid-event.json")
	os.WriteFile(invalidCalendar, []byte(`{"calendar_id":"primary","event_id":"fixture"}`), 0600)
	if err := a.Run(ctx, []string{"calendar", "preview", "--account", "personal", "--action", "delete", "--file", invalidCalendar}); err == nil || !strings.Contains(err.Error(), "send_updates") {
		t.Fatal("invalid calendar preview reached authentication", err)
	}
	if creds.calls != 0 {
		t.Fatal("invalid command touched credentials")
	}
}

func TestInvalidReadsFailBeforeAccountOrCredentialAccess(t *testing.T) {
	a, _, creds := fixture(t)
	for _, args := range [][]string{
		{"mail", "search", "--account", "unknown"},
		{"mail", "search", "--account", "unknown", "--query", "fixture", "--limit", "101"},
		{"mail", "get", "--account", "unknown"},
		{"draft", "get", "--account", "unknown"},
		{"calendar", "list", "--account", "unknown", "--limit", "0"},
		{"calendar", "events", "--account", "unknown", "--calendar", "primary", "--start", "bad", "--end", "bad"},
		{"calendar", "get", "--account", "unknown", "--id", "fixture"},
	} {
		if err := a.Run(context.Background(), args); err == nil || strings.Contains(err.Error(), "account alias") {
			t.Fatal("invalid read reached account resolution", args, err)
		}
	}
	if creds.calls != 0 {
		t.Fatal("invalid reads touched credentials")
	}
	if _, err := os.Stat(filepath.Dir(a.RegistryPath)); !os.IsNotExist(err) {
		t.Fatal("invalid reads created state", err)
	}
}
