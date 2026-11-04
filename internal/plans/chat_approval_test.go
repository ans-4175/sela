//go:build darwin || linux

package plans

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ans-4175/sela/internal/localstore"
)

func TestChatApprovalIsExactLocalAndWorksForAllActions(t *testing.T) {
	for _, action := range []string{"mail.send", "draft.create", "mail.mark-read", "calendar.create", "calendar.update", "calendar.delete"} {
		t.Run(action, func(t *testing.T) {
			ctx := context.Background()
			s := store(t)
			p, err := s.Create(ctx, fixture, action, map[string]string{"fixture": "exact"})
			if err != nil {
				t.Fatal(err)
			}
			for _, bad := range []ChatApproval{
				{}, {PlanHash: p.Hash, UserStatement: " \n\t"}, {PlanHash: strings.Repeat("0", 64), UserStatement: "Synthetic user consent"},
				{PlanHash: p.Hash, UserStatement: strings.Repeat("x", 4097)},
				{PlanHash: p.Hash, UserStatement: "Synthetic user consent", Reference: strings.Repeat("x", 513)},
				{PlanHash: p.Hash, UserStatement: string([]byte{0xff})},
			} {
				if s.Approve(ctx, p.Scope.ID, fixture, bad) == nil {
					t.Fatal("invalid consent accepted")
				}
			}
			unchanged, _ := s.Load(ctx, p.Scope.ID)
			if unchanged.State != "prepared" || unchanged.ApprovedAt != nil {
				t.Fatal("invalid consent consumed plan")
			}
			approval := ChatApproval{PlanHash: p.Hash, UserStatement: "Synthetic user requested this exact fixture action", Reference: "fixture conversation/request"}
			if err := s.Approve(ctx, p.Scope.ID, fixture, approval); err != nil {
				t.Fatal(err)
			}
			saved, err := s.Load(ctx, p.Scope.ID)
			if err != nil || saved.Approval == nil || saved.Approval.Method != "agent_attested_chat" || saved.Approval.ChatApproval != approval || saved.ApprovedAt == nil || saved.AttemptedAt != nil || saved.Hash != p.Hash {
				t.Fatal("receipt/scope changed", err)
			}
			if s.Approve(ctx, p.Scope.ID, fixture, approval) == nil {
				t.Fatal("approval overwritten")
			}
			_, err = s.Execute(ctx, p.Scope.ID, fixture, func(Plan) (json.RawMessage, error) { return json.RawMessage(`{"fixture":"done"}`), nil })
			if err != nil {
				t.Fatal(err)
			}
			if s.Approve(ctx, p.Scope.ID, fixture, approval) == nil {
				t.Fatal("succeeded plan reapproved")
			}
		})
	}
}

func TestChatApprovalLegacyAndReceiptIntegrity(t *testing.T) {
	ctx := context.Background()
	s := store(t)
	p, err := s.Create(ctx, fixture, "calendar.create", map[string]string{"fixture": "legacy"})
	if err != nil {
		t.Fatal(err)
	}
	// An already-approved legacy record stays readable; no invented chat evidence.
	now := time.Now().UTC()
	p.State = "approved"
	p.ApprovedAt = &now
	path := filepath.Join(s.Dir, p.Scope.ID+".json")
	if err := localstore.Write(path, p); err != nil {
		t.Fatal(err)
	}
	old, err := s.Load(ctx, p.Scope.ID)
	if err != nil || old.Approval != nil || old.State != "approved" {
		t.Fatal("legacy approval unreadable", err)
	}
	calls := 0
	result, err := s.Execute(ctx, p.Scope.ID, fixture, func(Plan) (json.RawMessage, error) { calls++; return json.RawMessage(`{"fixture":"done"}`), nil })
	if err != nil || calls != 1 || result.State != "succeeded" {
		t.Fatal("legacy attempt fence changed", err)
	}
	if _, err := s.Execute(ctx, p.Scope.ID, fixture, func(Plan) (json.RawMessage, error) { t.Error("legacy action replayed"); return nil, nil }); err == nil {
		t.Fatal("legacy replay allowed")
	}
	// An accidentally mismatched receipt must not be presented as approved.
	result.Approval = &ApprovalRecord{Method: "agent_attested_chat", ChatApproval: ChatApproval{PlanHash: strings.Repeat("0", 64), UserStatement: "Synthetic user consent"}}
	if err := localstore.Write(path, result); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(ctx, p.Scope.ID); err == nil {
		t.Fatal("mismatched approval receipt loaded")
	}
}
