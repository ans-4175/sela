package cli

import (
	"context"

	"github.com/ans-4175/sela/internal/accounts"
)

func (a *App) trashPreview(ctx context.Context, account accounts.Account, id string) error {
	client, _, err := a.client(ctx, account)
	if err != nil {
		return err
	}
	snapshot, err := client.TrashPreview(ctx, id)
	if err != nil {
		return err
	}
	store, err := a.planStore()
	if err != nil {
		return err
	}
	plan, err := store.Create(ctx, account, "mail.trash", snapshot)
	if err != nil {
		return err
	}
	return encode(a.Out, map[string]any{"account": account, "untrusted_remote_data": true, "trash_plan": plan, "changed": false, "notice": "Preview only: one exact Inbox message moves to Trash after actual user approval and separately consented --mail-modify. Trash may be permanently deleted by Gmail retention or account policy. Not a thread or query action."})
}
