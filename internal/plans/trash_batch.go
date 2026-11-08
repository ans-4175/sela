package plans

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/actions"
	"github.com/ans-4175/sela/internal/localstore"
)

type TrashBatchItem struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
type TrashBatchResult struct {
	Items  []TrashBatchItem `json:"items"`
	Notice string           `json:"notice"`
}

// ExecuteTrashBatch persists each item claim before its one provider attempt.
// Interrupted batches are never resumed; inspect plan show and reconcile first.
func (s Store) ExecuteTrashBatch(ctx context.Context, id string, a accounts.Account, perform func(actions.TrashBatch, int) error) (Plan, error) {
	var result Plan
	err := s.locked(ctx, func() error {
		p, err := s.load(id)
		if err != nil {
			return err
		}
		if err = s.valid(p, a); err != nil {
			return err
		}
		if p.Scope.Action != "mail.trash-batch" || p.State != "approved" || p.ApprovedAt == nil || p.AttemptedAt != nil || perform == nil {
			return errors.New("batch plan not executable; never replay an attempted plan")
		}
		var batch actions.TrashBatch
		if json.Unmarshal(p.Scope.Payload, &batch) != nil || batch.Validate() != nil {
			return errors.New("invalid batch snapshot; nothing attempted")
		}
		progress := TrashBatchResult{Items: make([]TrashBatchItem, len(batch.Messages)), Notice: "A claimed item may have an unknown outcome after a crash. Never replay; reconcile separately."}
		for i, m := range batch.Messages {
			progress.Items[i] = TrashBatchItem{ID: m.ID, Status: "not_attempted"}
		}
		path, _ := s.path(id)
		persist := func() error { p.Result, _ = json.Marshal(progress); return localstore.Write(path, p) }
		now := s.now()
		p.State = "executing"
		p.AttemptedAt = &now
		if err = persist(); err != nil {
			return errors.New("cannot persist batch claim; no provider call made")
		}
		result = p
		for i := range batch.Messages {
			if ctx.Err() != nil || s.valid(p, a) != nil {
				p.State = "partial"
				if persist() != nil {
					return errors.New("cannot persist stopped batch; do not retry")
				}
				result = p
				return errors.New("batch stopped before next item; remaining items not attempted, no replay")
			}
			progress.Items[i].Status = "in_flight"
			if persist() != nil {
				return errors.New("cannot persist item claim; stop and reconcile; no retry")
			}
			result = p
			if err = perform(batch, i); err != nil {
				progress.Items[i].Status = "unknown"
				p.State = "unknown"
				if persist() != nil {
					return errors.New("cannot persist item outcome; stop and reconcile; no retry")
				}
				result = p
				return errors.New("batch stopped: item outcome unknown; reconcile, never replay")
			}
			progress.Items[i].Status = "verified_trashed"
			if persist() != nil {
				return errors.New("cannot persist verified result; stop and reconcile; no retry")
			}
			result = p
		}
		p.State = "succeeded"
		if persist() != nil {
			return errors.New("cannot persist batch completion; stop and reconcile; no retry")
		}
		result = p
		return nil
	})
	return result, err
}
