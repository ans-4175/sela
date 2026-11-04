package cli

import (
	"context"
	"errors"

	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/actions"
	"github.com/ans-4175/sela/internal/googleapi"
	"github.com/ans-4175/sela/internal/plans"
)

func (a *App) markReadPlan(ctx context.Context, account accounts.Account, payload actions.MarkRead) (*plans.Plan, error) {
	if len(payload.Messages) == 0 {
		return nil, nil
	} // already READ; no write plan needed
	if err := payload.Validate(); err != nil {
		return nil, err
	}
	store, err := a.planStore()
	if err != nil {
		return nil, err
	}
	plan, err := store.Create(ctx, account, "mail.mark-read", payload)
	return &plan, err
}
func (a *App) markReadPreview(ctx context.Context, account accounts.Account, ids []string) error {
	client, _, err := a.client(ctx, account)
	if err != nil {
		return err
	}
	payload, skipped, err := client.MarkReadPreview(ctx, ids)
	if err != nil {
		return err
	}
	plan, err := a.markReadPlan(ctx, account, payload)
	if err != nil {
		return err
	}
	return encode(a.Out, map[string]any{"account": account, "untrusted_remote_data": true, "mark_read_plan": plan, "already_read_ids": skipped, "changed": false, "notice": "Preview only. Human plan approval and explicit --mail-modify consent are required before execution."})
}
func (a *App) readAndPreview(ctx context.Context, account accounts.Account, id string) error {
	client, _, err := a.client(ctx, account)
	if err != nil {
		return err
	}
	message, err := client.Message(ctx, id)
	if err != nil {
		return err
	}
	if message.Id != id {
		return errors.New("message ID mismatch; no mark-read plan prepared")
	}
	ref, unread := googleapi.MarkReadCandidate(message)
	payload := actions.MarkRead{}
	skipped := []string{}
	if unread {
		payload.Messages = []actions.MarkReadMessage{ref}
	} else {
		skipped = append(skipped, id)
	}
	plan, err := a.markReadPlan(ctx, account, payload)
	if err != nil {
		return err
	}
	return encode(a.Out, map[string]any{"account": account, "untrusted_remote_data": true, "data": message, "mark_read_plan": plan, "already_read_ids": skipped, "changed": false, "notice": "Message read without changing labels. Execute the human-approved mark_read_plan to mark READ."})
}
