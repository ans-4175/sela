//go:build darwin || linux

package plans

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ans-4175/sela/internal/actions"
)

func batchFixture() actions.TrashBatch {
	msg := func(id string) actions.Trash {
		return actions.Trash{ID: id, ThreadID: "thread", InternalDate: 1, From: "from", Subject: "subject", Date: "date", Labels: []string{"CATEGORY_PROMOTIONS", "INBOX"}}
	}
	return actions.TrashBatch{Source: "inbox", Category: "CATEGORY_PROMOTIONS", Messages: []actions.Trash{msg("alpha"), msg("beta"), msg("gamma")}}
}
func prepareBatch(t *testing.T, s Store) Plan {
	t.Helper()
	p, e := s.Create(context.Background(), fixture, "mail.trash-batch", batchFixture())
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Approve(context.Background(), p.Scope.ID, fixture, ChatApproval{PlanHash: p.Hash, UserStatement: "synthetic exact batch consent"}); e != nil {
		t.Fatal(e)
	}
	return p
}
func statuses(t *testing.T, p Plan) []TrashBatchItem {
	t.Helper()
	var r TrashBatchResult
	if err := json.Unmarshal(p.Result, &r); err != nil {
		t.Fatal(err)
	}
	return r.Items
}
func TestTrashBatchUnknownStopsAndPreservesProgress(t *testing.T) {
	s := store(t)
	p := prepareBatch(t, s)
	calls := 0
	result, err := s.ExecuteTrashBatch(context.Background(), p.Scope.ID, fixture, func(b actions.TrashBatch, i int) error {
		calls++
		onDisk, e := s.load(p.Scope.ID)
		if e != nil || statuses(t, onDisk)[i].Status != "in_flight" {
			t.Fatal("claim not durable")
		}
		if i == 1 {
			return errors.New("synthetic")
		}
		return nil
	})
	if err == nil || calls != 2 || result.State != "unknown" {
		t.Fatal(calls, result, err)
	}
	got, _ := s.Load(context.Background(), p.Scope.ID)
	v := statuses(t, got)
	if v[0].Status != "verified_trashed" || v[1].Status != "unknown" || v[2].Status != "not_attempted" {
		t.Fatal(v)
	}
	if _, err = s.ExecuteTrashBatch(context.Background(), p.Scope.ID, fixture, func(actions.TrashBatch, int) error { t.Fatal("replay"); return nil }); err == nil {
		t.Fatal("replayed")
	}
}
func TestTrashBatchCrashAndConcurrentClaim(t *testing.T) {
	s := store(t)
	p := prepareBatch(t, s)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("no synthetic crash")
			}
		}()
		s.ExecuteTrashBatch(context.Background(), p.Scope.ID, fixture, func(actions.TrashBatch, int) error { panic("fixture crash") })
	}()
	got, _ := s.Load(context.Background(), p.Scope.ID)
	if got.State != "executing" || statuses(t, got)[0].Status != "in_flight" {
		t.Fatal(got)
	}
	if _, err := s.ExecuteTrashBatch(context.Background(), p.Scope.ID, fixture, func(actions.TrashBatch, int) error { t.Fatal("replay"); return nil }); err == nil {
		t.Fatal("replayed crash")
	}
	p = prepareBatch(t, s)
	var writes atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.ExecuteTrashBatch(context.Background(), p.Scope.ID, fixture, func(actions.TrashBatch, int) error { writes.Add(1); return nil })
		}()
	}
	wg.Wait()
	got, _ = s.Load(context.Background(), p.Scope.ID)
	if writes.Load() != 3 || got.State != "succeeded" {
		t.Fatal(writes.Load(), got.State)
	}
}
func TestTrashBatchExpiryStopsBeforeNext(t *testing.T) {
	s := store(t)
	now := time.Now()
	s.Now = func() time.Time { return now }
	p := prepareBatch(t, s)
	got, err := s.ExecuteTrashBatch(context.Background(), p.Scope.ID, fixture, func(actions.TrashBatch, int) error { now = now.Add(16 * time.Minute); return nil })
	v := statuses(t, got)
	if err == nil || got.State != "partial" || v[0].Status != "verified_trashed" || v[1].Status != "not_attempted" {
		t.Fatal(got, err)
	}
}
