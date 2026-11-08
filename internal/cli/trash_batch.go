package cli

import (
	"context"
	"errors"
	"time"

	"github.com/ans-4175/sela/internal/accounts"
)

func (a *App) trashBatchPreview(ctx context.Context, account accounts.Account, ids []string, source, category string) error {
	// Bound the entire preview, not just each provider request.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	client, _, err := a.client(ctx, account)
	if err != nil {
		return err
	}
	batch, skipped, err := client.TrashBatchPreview(ctx, ids, source, category)
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return errors.New("batch preview cancelled or timed out; no plan prepared")
	}
	var plan any
	if len(batch.Messages) > 0 {
		store, e := a.planStore()
		if e != nil {
			return e
		}
		p, e := store.Create(ctx, account, "mail.trash-batch", batch)
		if e != nil {
			return e
		}
		plan = p
	}
	return encode(a.Out, map[string]any{"account": account, "requested_count": len(ids), "eligible_count": len(batch.Messages), "skipped": skipped, "trash_batch_plan": plan, "untrusted_remote_data": true, "changed": false, "notice": "Exact selected messages only, one write each after explicit approval. Trash may later be permanently deleted. Stop on uncertainty; no automatic retry or continuation."})
}
