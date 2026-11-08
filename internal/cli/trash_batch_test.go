//go:build darwin || linux

package cli

import (
	"context"
	"encoding/json"
	"fmt"
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

func TestTrashBatchCLIApprovalAndStopOnUnknown(t *testing.T) {
	ctx := context.Background()
	app, out, _ := fixture(t)
	account := accounts.Account{Name: "personal", Email: "person@example.test"}
	if e := accounts.Add(ctx, app.RegistryPath, account); e != nil {
		t.Fatal(e)
	}
	creds := &validFixtureCredential{}
	app.Credentials = creds
	client := json.RawMessage(`{"installed":{"client_id":"synthetic","client_secret":"SYNTHETIC_SECRET","auth_uri":"` + google.Endpoint.AuthURL + `","token_uri":"` + google.Endpoint.TokenURL + `","redirect_uris":["http://localhost"]}}`)
	conn := Connection{Ready: true, Client: client}
	dir, _, _ := app.paths()
	path := filepath.Join(dir, "personal.json")
	if e := localstore.Write(path, conn); e != nil {
		t.Fatal(e)
	}
	old := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = old })
	calls, writes := 0, 0
	http.DefaultTransport = markReadTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := `{}`
		status := 200
		if strings.Contains(r.URL.Path, "userinfo") {
			body = `{"email":"person@example.test","verified_email":true}`
		} else if r.Method == "POST" {
			writes++
			status = 503
		} else {
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			body = fmt.Sprintf(`{"id":%q,"threadId":"thread1","internalDate":"1234","labelIds":["INBOX","CATEGORY_PROMOTIONS"],"payload":{"headers":[{"name":"From","value":"fixture@example.test"},{"name":"Subject","value":"fixture"},{"name":"Date","value":"fixture"}]}}`, id)
		}
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	invalid := []string{"mail", "preview", "--account", "personal", "--action", "trash-batch", "--source", "inbox", "--category", "CATEGORY_PROMOTIONS", "--ids", "a,a"}
	if e := app.Run(ctx, invalid); e == nil || calls != 0 || creds.loads != 0 {
		t.Fatal("invalid ids accessed provider", e)
	}
	out.Reset()
	if e := app.Run(ctx, []string{"mail", "preview", "--account", "personal", "--action", "trash-batch", "--source", "inbox", "--category", "CATEGORY_PROMOTIONS", "--ids", "alpha,beta"}); e != nil {
		t.Fatal(e)
	}
	var preview struct {
		Plan plans.Plan `json:"trash_batch_plan"`
	}
	if e := json.Unmarshal(out.Bytes(), &preview); e != nil {
		t.Fatal(e)
	}
	p := preview.Plan
	if p.State != "prepared" || p.Scope.Action != "mail.trash-batch" || writes != 0 {
		t.Fatal("preview", p)
	}
	if e := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", p.Scope.ID}); e == nil {
		t.Fatal("unapproved")
	}
	if e := app.Run(ctx, []string{"plan", "approve", "--account", "personal", "--id", p.Scope.ID, "--hash", p.Hash, "--user-approval", "Synthetic user approves exact batch"}); e != nil {
		t.Fatal(e)
	}
	before, loads := calls, creds.loads
	if e := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", p.Scope.ID}); e == nil || calls != before || creds.loads != loads {
		t.Fatal("scope check", e)
	}
	conn.MailModify = true
	if e := localstore.Write(path, conn); e != nil {
		t.Fatal(e)
	}
	if e := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", p.Scope.ID}); e == nil || writes != 1 {
		t.Fatal("unknown not stopped", e, writes)
	}
	store, _ := app.planStore()
	saved, e := store.Load(ctx, p.Scope.ID)
	if e != nil || saved.State != "unknown" || !strings.Contains(string(saved.Result), `"not_attempted"`) {
		t.Fatal(saved, e)
	}
	if e := app.Run(ctx, []string{"plan", "execute", "--account", "personal", "--id", p.Scope.ID}); e == nil || writes != 1 {
		t.Fatal("replayed", e, writes)
	}
}
