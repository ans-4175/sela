//go:build darwin || linux

package plans

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/localstore"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var fixture = accounts.Account{Name: "personal", Email: "person@example.test"}

func store(t *testing.T) Store {
	dir, e := filepath.EvalSymlinks(t.TempDir())
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if e != nil {
		t.Fatal(e)
	}
	return Store{Dir: dir}
}
func TestApprovalIntegrityExpiryAndAccount(t *testing.T) {
	s := store(t)
	now := time.Date(2026, 11, 4, 0, 0, 0, 0, time.UTC)
	s.Now = func() time.Time { return now }
	if _, err := s.Create(context.Background(), fixture, "draft.update", map[string]string{}); err == nil {
		t.Fatal("unsupported draft update action accepted")
	}
	p, e := s.Create(context.Background(), fixture, "mail.send", map[string]string{"body": "exact"})
	if e != nil {
		t.Fatal(e)
	}
	if s.Approve(context.Background(), p.Scope.ID, accounts.Account{Name: "other", Email: "other@example.test"}, ChatApproval{PlanHash: p.Hash, UserStatement: "Synthetic user approval"}) == nil {
		t.Fatal("cross account")
	}
	if s.Approve(context.Background(), p.Scope.ID, fixture, ChatApproval{}) == nil {
		t.Fatal("missing chat consent accepted")
	}
	now = now.Add(15 * time.Minute)
	if s.Approve(context.Background(), p.Scope.ID, fixture, ChatApproval{PlanHash: p.Hash, UserStatement: "Synthetic user approval"}) == nil {
		t.Fatal("expiry during confirmation ignored")
	}
	now = now.Add(-time.Minute)
	p.Scope.Payload = json.RawMessage(`{"body":"changed"}`)
	localstore.Write(filepath.Join(s.Dir, p.Scope.ID+".json"), p)
	if _, e = s.Load(context.Background(), p.Scope.ID); e == nil {
		t.Fatal("tampering accepted")
	}
	if _, e = s.Load(context.Background(), "../escape"); e == nil {
		t.Fatal("path escape")
	}
}
func TestOneAttemptConcurrentExecution(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	p, e := s.Create(ctx, fixture, "mail.send", map[string]string{"body": "exact"})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Execute(ctx, p.Scope.ID, fixture, func(Plan) (json.RawMessage, error) { t.Error("unapproved transport"); return nil, nil }); e == nil {
		t.Fatal("unapproved execution")
	}
	if e = s.Approve(ctx, p.Scope.ID, fixture, ChatApproval{PlanHash: p.Hash, UserStatement: "Synthetic user approval"}); e != nil {
		t.Fatal(e)
	}
	var calls atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Execute(ctx, p.Scope.ID, fixture, func(got Plan) (json.RawMessage, error) {
				calls.Add(1)
				var disk Plan
				if e := localstore.Read(filepath.Join(s.Dir, p.Scope.ID+".json"), &disk); e != nil || disk.State != "executing" || disk.AttemptedAt == nil {
					t.Error("claim was not durable before transport", e)
				}
				return json.RawMessage(`{"id":"synthetic"}`), nil
			})
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatal("replay", calls.Load())
	}
	got, e := s.Load(ctx, p.Scope.ID)
	if e != nil || got.State != "succeeded" {
		t.Fatal(got, e)
	}
}
func TestUnknownAndClaimedCrashNeverRetry(t *testing.T) {
	s := store(t)
	ctx := context.Background()
	for _, crash := range []bool{false, true} {
		p, e := s.Create(ctx, fixture, "calendar.create", map[string]string{"summary": "fixture"})
		if e != nil {
			t.Fatal(e)
		}
		s.Approve(ctx, p.Scope.ID, fixture, ChatApproval{PlanHash: p.Hash, UserStatement: "Synthetic user approval"})
		if crash {
			p, _ = s.Load(ctx, p.Scope.ID)
			now := time.Now()
			p.State = "executing"
			p.AttemptedAt = &now
			localstore.Write(filepath.Join(s.Dir, p.Scope.ID+".json"), p)
		} else {
			p, e = s.Execute(ctx, p.Scope.ID, fixture, func(Plan) (json.RawMessage, error) { return nil, errors.New("SYNTHETIC_PRIVATE_RESPONSE") })
			if e == nil || p.State != "unknown" {
				t.Fatal(e, p)
			}
		}
		if s.Approve(ctx, p.Scope.ID, fixture, ChatApproval{PlanHash: p.Hash, UserStatement: "Synthetic user consent"}) == nil {
			t.Fatal("unknown/executing plan reapproved")
		}
		if _, e = s.Execute(ctx, p.Scope.ID, fixture, func(Plan) (json.RawMessage, error) { t.Error("ambiguous attempt retried"); return nil, nil }); e == nil {
			t.Fatal("replayed")
		}
	}
}
