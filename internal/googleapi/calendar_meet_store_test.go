//go:build darwin || linux

package googleapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/plans"
)

func TestMeetPendingReceiptPersistsWithoutReplay(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store := plans.Store{Dir: filepath.Join(root, "plans")}
	account := accounts.Account{Name: "personal", Email: "person@example.test"}
	plan, err := store.Create(ctx, account, "calendar.create", meetFixture())
	if err != nil {
		t.Fatal(err)
	}
	// Synthetic chat fixture only; no actual user/Keychain/provider interaction.
	if err := store.Approve(ctx, plan.Scope.ID, account, plans.ChatApproval{PlanHash: plan.Hash, UserStatement: "Synthetic user requested this fixture event with Meet"}); err != nil {
		t.Fatal(err)
	}
	calls, posts := 0, 0
	c, _ := NewWithHTTP(ctx, &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method == "POST" {
			posts++
			return reply(200, `{"id":"fixture","etag":"insert-etag"}`), nil
		}
		b, _ := json.Marshal(conferenceReply("pending"))
		return reply(200, string(b)), nil
	})})
	result, err := store.Execute(ctx, plan.Scope.ID, account, func(p plans.Plan) (json.RawMessage, error) { return c.Perform(ctx, p) })
	if err != nil || result.State != "succeeded" {
		t.Fatal("known event create discarded", err)
	}
	saved, err := store.Load(ctx, plan.Scope.ID)
	if err != nil {
		t.Fatal(err)
	}
	var receipt meetCreateResult
	json.Unmarshal(saved.Result, &receipt)
	if !receipt.EventCreated || receipt.EventID != "fixture" || receipt.ConferenceStatus != "pending" || receipt.MeetReady || receipt.WorkflowComplete || saved.AttemptedAt == nil {
		t.Fatal("pending receipt lost or reported complete")
	}
	if _, err := store.Execute(ctx, plan.Scope.ID, account, func(p plans.Plan) (json.RawMessage, error) { return c.Perform(ctx, p) }); err == nil {
		t.Fatal("pending event allowed replay")
	}
	if posts != 1 || calls != 2 {
		t.Fatal("partial success caused retry", posts, calls)
	}
}
