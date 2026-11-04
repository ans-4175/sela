package cli

import (
	"context"
	"errors"
	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/actions"
	"github.com/ans-4175/sela/internal/googleapi"
)

func (a *App) calendarPreview(ctx context.Context, account accounts.Account, action, file string) error {
	kind := "calendar." + action
	if action != "create" && action != "update" && action != "delete" {
		return errors.New("calendar action create|update|delete required")
	}
	var cp actions.Calendar
	if e := readInput(file, &cp); e != nil {
		return e
	}
	if cp.Before != nil {
		return errors.New("before snapshot is captured by sela, not accepted from input")
	}
	if err := cp.ValidateInput(kind); err != nil {
		return err
	}
	if action != "create" {
		client, _, e := a.client(ctx, account)
		if e != nil {
			return e
		}
		current, e := client.Event(ctx, cp.CalendarID, cp.EventID)
		if e != nil {
			return e
		}
		if current.RecurringEventId != "" || len(current.Recurrence) > 0 || current.Start == nil || current.Start.Date != "" {
			return errors.New("recurring/all-day event mutation is not supported in MVP")
		}
		if cp.ETag != "" && cp.ETag != current.Etag {
			return errors.New("event changed since selected ETag; refresh preview")
		}
		cp.ETag = current.Etag
		cp.Before = googleapi.EventView(current)
	}
	if e := cp.Validate(kind); e != nil {
		return e
	}
	store, e := a.planStore()
	if e != nil {
		return e
	}
	p, e := store.Create(ctx, account, kind, cp)
	if e != nil {
		return e
	}
	return encode(a.Out, map[string]any{"plan": p, "effects": "Attendees in before/event may receive invitations, updates or cancellations; send_updates none is a request, not a guarantee of no notifications. Event responses may be affected."})
}
