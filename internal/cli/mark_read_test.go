//go:build darwin || linux

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/auth"
	"github.com/ans-4175/sela/internal/localstore"
	"github.com/ans-4175/sela/internal/plans"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type markReadTransport func(*http.Request) (*http.Response, error)

func (f markReadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type validFixtureCredential struct{ loads int }

func (s *validFixtureCredential) Load(string) (*oauth2.Token, error) {
	s.loads++
	return &oauth2.Token{AccessToken: "SYNTHETIC_ACCESS", RefreshToken: "SYNTHETIC_REFRESH", Expiry: time.Now().Add(time.Hour)}, nil
}
func (s *validFixtureCredential) Save(string, *oauth2.Token) error {
	return errors.New("unexpected fixture token write")
}

func TestReadAndBulkMarkReadStayApprovalAndScopeBound(t *testing.T) {
	ctx := context.Background()
	app, out, _ := fixture(t)
	account := accounts.Account{Name: "personal", Email: "person@example.test"}
	if err := accounts.Add(ctx, app.RegistryPath, account); err != nil {
		t.Fatal(err)
	}
	creds := &validFixtureCredential{}
	app.Credentials = creds
	clientJSON := json.RawMessage(`{"installed":{"client_id":"synthetic","client_secret":"SYNTHETIC_SECRET","auth_uri":"` + google.Endpoint.AuthURL + `","token_uri":"` + google.Endpoint.TokenURL + `","redirect_uris":["http://localhost"]}}`)
	connection := Connection{Ready: true, Client: clientJSON}
	dir, _, _ := app.paths()
	connectionPath := filepath.Join(dir, "personal.json")
	if err := localstore.Write(connectionPath, connection); err != nil {
		t.Fatal(err)
	}
	oldTransport := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = oldTransport })
	calls, posts := 0, 0
	marked := false
	http.DefaultTransport = markReadTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{}`
		status := 200
		if strings.Contains(r.URL.Path, "userinfo") {
			body = `{"email":"person@example.test","verified_email":true}`
		} else if r.Method == "POST" {
			posts++
			marked = true
			status = 204
		} else {
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			labels := []string{"INBOX"}
			if id == "alpha" && !marked {
				labels = append(labels, "UNREAD")
			}
			encoded, _ := json.Marshal(map[string]any{"id": id, "labelIds": labels, "payload": map[string]any{"mimeType": "text/plain", "body": map[string]string{"data": "Zml4dHVyZQ"}, "headers": []map[string]string{{"name": "Subject", "value": "Fixture"}}}})
			body = string(encoded)
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	run := func(args ...string) map[string]json.RawMessage {
		t.Helper()
		out.Reset()
		if err := app.Run(ctx, args); err != nil {
			t.Fatal(args, err)
		}
		var value map[string]json.RawMessage
		if err := json.Unmarshal(out.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	normal := run("mail", "get", "--account", "personal", "--id", "alpha")
	if _, exists := normal["mark_read_plan"]; exists || posts != 0 {
		t.Fatal("default read changed behavior")
	}
	read := run("mail", "get", "--account", "personal", "--id", "alpha", "--mark-read")
	var plan plans.Plan
	if err := json.Unmarshal(read["mark_read_plan"], &plan); err != nil || plan.State != "prepared" || plan.Scope.Action != "mail.mark-read" || len(read["data"]) == 0 {
		t.Fatal("read+preview contract", err)
	}
	bulk := run("mail", "preview", "--account", "personal", "--action", "mark-read", "--ids", "alpha,beta")
	var skipped []string
	json.Unmarshal(bulk["already_read_ids"], &skipped)
	if len(skipped) != 1 || skipped[0] != "beta" || posts != 0 {
		t.Fatal("bulk preview did not skip read message")
	}
	already := run("mail", "get", "--account", "personal", "--id", "beta", "--mark-read")
	if string(already["mark_read_plan"]) != "null" {
		t.Fatal("already read message got a write plan")
	}
	legacy, cfg, err := app.connection(account)
	if err != nil || legacy.MailModify || strings.Contains(strings.Join(cfg.Scopes, " "), auth.MailModifyScope) {
		t.Fatal("legacy connection silently widened")
	}
	run("plan", "approve", "--account", "personal", "--id", plan.Scope.ID, "--hash", plan.Hash, "--user-approval", "Synthetic user requested marking only this fixture message READ")
	beforeCalls, beforeLoads := calls, creds.loads
	if err := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", plan.Scope.ID}); err == nil || !strings.Contains(err.Error(), "--mail-modify") {
		t.Fatal("missing scope not rejected", err)
	}
	if calls != beforeCalls || creds.loads != beforeLoads {
		t.Fatal("missing scope touched credentials/API")
	}
	store, _ := app.planStore()
	saved, _ := store.Load(ctx, plan.Scope.ID)
	if saved.State != "approved" || saved.AttemptedAt != nil {
		t.Fatal("scope failure consumed approval")
	}
	// Simulated successful consent in this private fixture only. No real Keychain/Google.
	connection.MailModify = true
	if err := localstore.Write(connectionPath, connection); err != nil {
		t.Fatal(err)
	}
	run("plan", "execute", "--account", "personal", "--id", plan.Scope.ID)
	saved, _ = store.Load(ctx, plan.Scope.ID)
	if saved.State != "succeeded" || posts != 1 {
		t.Fatal("approved mark-read execution", saved.State, posts)
	}
	if err := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", plan.Scope.ID}); err == nil || posts != 1 {
		t.Fatal("mark-read plan replayed")
	}
}
