package cli

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/actions"
	"github.com/ans-4175/sela/internal/plans"
	"time"
)

func (a *App) planCommand(ctx context.Context, cmd string, account accounts.Account, id string, approval plans.ChatApproval) error {
	store, e := a.planStore()
	if e != nil {
		return e
	}
	p, e := store.Load(ctx, id)
	if e != nil {
		return e
	}
	if p.Scope.Account != account {
		return errors.New("plan account mismatch")
	}
	if cmd == "show" {
		return encode(a.Out, p)
	}
	if cmd == "approve" {
		if e = store.Approve(ctx, id, account, approval); e != nil {
			return e
		}
		p, e = store.Load(ctx, id)
		if e != nil {
			return e
		}
		return encode(a.Out, p)
	}
	if p.State != "approved" || !time.Now().Before(p.Scope.Expires) {
		return errors.New("plan must be approved, unattempted and unexpired before credential/API access")
	}
	if p.Scope.Action == "mail.mark-read" || p.Scope.Action == "mail.trash" || p.Scope.Action == "mail.trash-batch" {
		conn, _, err := a.connection(account)
		if err != nil {
			return err
		}
		if !conn.MailModify {
			return errors.New("mail mutation requires explicit --mail-modify consent; nothing attempted")
		}
	}
	client, conn, e := a.client(ctx, account)
	if e != nil {
		return e
	}
	if (p.Scope.Action == "mail.mark-read" || p.Scope.Action == "mail.trash" || p.Scope.Action == "mail.trash-batch") && !conn.MailModify {
		return errors.New("mail mutation requires explicit --mail-modify consent; nothing attempted")
	}
	if (p.Scope.Action == "calendar.create" || p.Scope.Action == "calendar.update" || p.Scope.Action == "calendar.delete") && !conn.CalendarWrite {
		return errors.New("calendar write profile was not explicitly consented; nothing attempted")
	}
	if p.Scope.Action == "mail.trash-batch" {
		result, err := store.ExecuteTrashBatch(ctx, id, account, func(batch actions.TrashBatch, index int) error {
			_, err := client.TrashBatchItem(ctx, batch, index)
			return err
		})
		if err != nil {
			return err
		}
		return encode(a.Out, result)
	}
	result, e := store.Execute(ctx, id, account, func(p plans.Plan) (json.RawMessage, error) { return client.Perform(ctx, p) })
	if e != nil {
		return e
	}
	return encode(a.Out, result)
}
