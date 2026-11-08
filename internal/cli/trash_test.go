//go:build darwin || linux

package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/localstore"
	"github.com/ans-4175/sela/internal/plans"
	"golang.org/x/oauth2/google"
)

func TestTrashCLIConsentScopeAndNoReplay(t *testing.T) {
	ctx := context.Background()
	app, out, _ := fixture(t)
	account := accounts.Account{Name: "personal", Email: "person@example.test"}
	if err := accounts.Add(ctx, app.RegistryPath, account); err != nil {
		t.Fatal(err)
	}
	creds := &validFixtureCredential{}
	app.Credentials = creds
	client := json.RawMessage(`{"installed":{"client_id":"synthetic","client_secret":"SYNTHETIC_SECRET","auth_uri":"` + google.Endpoint.AuthURL + `","token_uri":"` + google.Endpoint.TokenURL + `","redirect_uris":["http://localhost"]}}`)
	connection := Connection{Ready: true, Client: client}
	dir, _, _ := app.paths()
	path := filepath.Join(dir, "personal.json")
	if err := localstore.Write(path, connection); err != nil {
		t.Fatal(err)
	}
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	writes, calls := 0, 0
	http.DefaultTransport = markReadTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{}`
		if strings.Contains(r.URL.Path, "userinfo") {
			body = `{"email":"person@example.test","verified_email":true}`
		} else if r.Method == "POST" {
			writes++
			if r.URL.Path != "/gmail/v1/users/me/messages/alpha/trash" {
				t.Fatal("wrong endpoint", r.URL.Path)
			}
			body = `{"id":"alpha"}`
		} else if writes > 0 {
			body = `{"id":"alpha","labelIds":["TRASH"]}`
		} else {
			body = `{"id":"alpha","threadId":"thread1","internalDate":"1234","labelIds":["INBOX"],"payload":{"headers":[{"name":"From","value":"fixture@example.test"},{"name":"Subject","value":"fixture"},{"name":"Date","value":"fixture"}]}}`
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	for _, args := range [][]string{{"mail", "preview", "--account", "personal", "--action", "trash", "--id", "alpha", "--ids", "beta"}, {"mail", "preview", "--account", "personal", "--action", "trash", "--id", "alpha", "--file", "foo"}, {"mail", "preview", "--account", "personal", "--action", "trash", "--ids", "alpha,beta"}} {
		if err := app.Run(ctx, args); err == nil || calls != 0 || creds.loads != 0 {
			t.Fatal("invalid preview touched provider", args, err)
		}
	}
	out.Reset()
	if err := app.Run(ctx, []string{"mail", "preview", "--account", "personal", "--action", "trash", "--id", "alpha"}); err != nil {
		t.Fatal(err)
	}
	var preview struct {
		Plan plans.Plan `json:"trash_plan"`
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	p := preview.Plan
	if p.State != "prepared" || p.Scope.Action != "mail.trash" || writes != 0 {
		t.Fatal("preview", p)
	}
	if err := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", p.Scope.ID}); err == nil || writes != 0 {
		t.Fatal("unapproved execution", err)
	}
	if err := app.Run(ctx, []string{"plan", "approve", "--account", "personal", "--id", p.Scope.ID, "--hash", p.Hash, "--user-approval", "Synthetic user approved the exact fixture message to Trash"}); err != nil {
		t.Fatal(err)
	}
	beforeCalls, beforeLoads := calls, creds.loads
	if err := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", p.Scope.ID}); err == nil || calls != beforeCalls || creds.loads != beforeLoads {
		t.Fatal("missing scope accessed API", err)
	}
	connection.MailModify = true
	if err := localstore.Write(path, connection); err != nil {
		t.Fatal(err)
	}
	if err := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", p.Scope.ID}); err != nil {
		t.Fatal(err)
	}
	store, _ := app.planStore()
	saved, err := store.Load(ctx, p.Scope.ID)
	if err != nil || saved.State != "succeeded" || !strings.Contains(string(saved.Result), "verified_trashed") || writes != 1 {
		t.Fatal(saved, err, writes)
	}
	if err := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", p.Scope.ID}); err == nil || writes != 1 {
		t.Fatal("replay", err, writes)
	}

	// A fresh plan becomes protected between preview and execution. The claim is
	// conservatively unknown even though the synthetic provider saw no write.
	writes = 0
	out.Reset()
	if err := app.Run(ctx, []string{"mail", "preview", "--account", "personal", "--action", "trash", "--id", "alpha"}); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	p = preview.Plan
	if err := app.Run(ctx, []string{"plan", "approve", "--account", "personal", "--id", p.Scope.ID, "--hash", p.Hash, "--user-approval", "Synthetic user approved this exact fixture"}); err != nil {
		t.Fatal(err)
	}
	prior := http.DefaultTransport
	http.DefaultTransport = markReadTransport(func(r *http.Request) (*http.Response, error) {
		if strings.Contains(r.URL.Path, "userinfo") {
			return prior.RoundTrip(r)
		}
		if r.Method == "POST" {
			t.Fatal("stale message written")
		}
		body := `{"id":"alpha","threadId":"thread1","internalDate":"1234","labelIds":["INBOX","STARRED"],"payload":{"headers":[{"name":"From","value":"fixture@example.test"},{"name":"Subject","value":"fixture"},{"name":"Date","value":"fixture"}]}}`
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	if err := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", p.Scope.ID}); err == nil {
		t.Fatal("stale execution succeeded")
	}
	saved, err = store.Load(ctx, p.Scope.ID)
	if err != nil || saved.State != "unknown" || saved.AttemptedAt == nil {
		t.Fatal("stale claim", saved, err)
	}
	if err := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", p.Scope.ID}); err == nil {
		t.Fatal("stale plan replayed")
	}
}
