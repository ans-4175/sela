package googleapi

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/ans-4175/sela/internal/actions"
)

type TrashSkipped struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

func (c *Clients) trashPreviewFor(ctx context.Context, id, source, category string) (actions.Trash, error) {
	if err := actions.ValidateMessageIDs([]string{id}); err != nil {
		return actions.Trash{}, err
	}
	m, err := c.readMail.Users.Messages.Get("me", id).Format("metadata").MetadataHeaders("From", "Subject", "Date").Fields("id,threadId,internalDate,labelIds,payload/headers").Context(ctx).Do()
	if err != nil {
		return actions.Trash{}, safe(err)
	}
	snapshot, err := trashSnapshot(m, id)
	if err != nil {
		return actions.Trash{}, err
	}
	if err = snapshot.ValidateFor(source, category); err != nil {
		return actions.Trash{}, err
	}
	return snapshot, nil
}
func (c *Clients) TrashBatchPreview(ctx context.Context, ids []string, source, category string) (actions.TrashBatch, []TrashSkipped, error) {
	b := actions.TrashBatch{Source: source, Category: category, Messages: []actions.Trash{}}
	skipped := []TrashSkipped{}
	if err := actions.ValidateTrashSource(source, category); err != nil {
		return b, nil, err
	}
	if err := actions.ValidateTrashBatchIDs(ids); err != nil {
		return b, nil, err
	}
	for _, id := range ids {
		m, err := c.readMail.Users.Messages.Get("me", id).Format("metadata").MetadataHeaders("From", "Subject", "Date").Fields("id,threadId,internalDate,labelIds,payload/headers").Context(ctx).Do()
		if err != nil {
			return b, nil, safe(err)
		}
		snap, err := trashSnapshot(m, id)
		if err != nil {
			return b, nil, err
		}
		if err = snap.ValidateFor(source, category); err != nil {
			skipped = append(skipped, TrashSkipped{ID: id, Reason: err.Error()})
			continue
		}
		b.Messages = append(b.Messages, snap)
	}
	if len(b.Messages) > 0 {
		if err := b.Validate(); err != nil {
			return b, nil, err
		}
	}
	return b, skipped, nil
}

// TrashBatchItem is called only by the durable batch executor for a claimed item.
func (c *Clients) TrashBatchItem(ctx context.Context, b actions.TrashBatch, index int) (json.RawMessage, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	if index < 0 || index >= len(b.Messages) {
		return nil, errors.New("invalid batch index")
	}
	return c.trashFor(ctx, b.Messages[index], b.Source, b.Category)
}
