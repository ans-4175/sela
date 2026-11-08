//go:build darwin || linux

package plans

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ans-4175/sela/internal/actions"
)

func largeBatch() actions.TrashBatch {
	b := batchFixture()
	b.Messages = nil
	for i := 0; i < 500; i++ {
		m := batchFixture().Messages[0]
		m.ID = fmt.Sprintf("id%03d", i)
		b.Messages = append(b.Messages, m)
	}
	return b
}
func TestTrashBatch500DurableProgress(t *testing.T) {
	for _, mode := range []string{"success", "unknown", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			s := store(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			p, e := s.Create(ctx, fixture, "mail.trash-batch", largeBatch())
			if e != nil {
				t.Fatal(e)
			}
			if e = s.Approve(ctx, p.Scope.ID, fixture, ChatApproval{PlanHash: p.Hash, UserStatement: "Synthetic explicit 500-item approval"}); e != nil {
				t.Fatal(e)
			}
			calls := 0
			got, e := s.ExecuteTrashBatch(ctx, p.Scope.ID, fixture, func(b actions.TrashBatch, i int) error {
				calls++
				if i == 0 || i == 249 || i == 499 {
					disk, err := s.load(p.Scope.ID)
					if err != nil || statuses(t, disk)[i].Status != "in_flight" {
						t.Fatal("missing durable item claim", i, err)
					}
				}
				if mode == "unknown" && i == 249 {
					return errors.New("fixture provider timeout")
				}
				if mode == "cancel" && i == 249 {
					cancel()
				}
				return nil
			})
			result := statuses(t, got)
			switch mode {
			case "success":
				if e != nil || calls != 500 || got.State != "succeeded" || result[499].Status != "verified_trashed" {
					t.Fatal(e, calls, got.State)
				}
			case "unknown":
				if e == nil || calls != 250 || got.State != "unknown" || result[248].Status != "verified_trashed" || result[249].Status != "unknown" || result[250].Status != "not_attempted" {
					t.Fatal(e, calls, got.State)
				}
			case "cancel":
				if e == nil || calls != 250 || got.State != "partial" || result[249].Status != "verified_trashed" || result[250].Status != "not_attempted" {
					t.Fatal(e, calls, got.State)
				}
			}
			if _, err := s.ExecuteTrashBatch(context.Background(), p.Scope.ID, fixture, func(actions.TrashBatch, int) error { t.Fatal("batch replayed"); return nil }); err == nil {
				t.Fatal("replay accepted")
			}
		})
	}
}
