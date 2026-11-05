package cli

import (
	"context"
	"errors"
	"github.com/ans-4175/sela/internal/googleapi"
)

func (a *App) readWorkflow(ctx context.Context, args []string) error {
	alias, file, id := "", "", ""
	limit := int64(10)
	allowed := map[string]*string{"account": &alias}
	numbers := map[string]*int64{}
	offset := 2
	mode := args[0]
	if mode == "brief" {
		offset = 1
		allowed["file"] = &file
	} else if mode == "calendar" {
		mode = "availability"
		allowed["file"] = &file
	} else {
		mode = "thread"
		allowed["id"] = &id
		numbers["limit"] = &limit
	}
	if err := parse(mode, args[offset:], allowed, numbers, nil); err != nil {
		return err
	}
	var availability googleapi.AvailabilityInput
	var brief googleapi.BriefInput
	// Parse/validate everything before resolving an account or opening the keyring.
	switch mode {
	case "thread":
		if err := googleapi.ValidateThread(id, limit); err != nil {
			return err
		}
	case "availability":
		if err := readInput(file, &availability); err != nil {
			return err
		}
		if err := availability.Validate(); err != nil {
			return err
		}
	case "brief":
		if err := readInput(file, &brief); err != nil {
			return err
		}
		if err := brief.Validate(); err != nil {
			return err
		}
	}
	acct, err := a.account(alias)
	if err != nil {
		return err
	}
	if mode == "availability" {
		conn, _, err := a.connection(acct)
		if err != nil {
			return err
		}
		if !conn.CalendarFreeBusy {
			return errors.New("availability requires explicit --calendar-freebusy login opt-in and fresh browser consent; existing scopes are unchanged")
		}
	}
	client, _, err := a.client(ctx, acct)
	if err != nil {
		return err
	}
	var response any
	switch mode {
	case "thread":
		response, err = client.Thread(ctx, id, limit)
	case "availability":
		response, err = client.Availability(ctx, availability)
	case "brief":
		response, err = client.Brief(ctx, brief)
	}
	if err != nil {
		return err
	}
	return encode(a.Out, map[string]any{"account": acct, "untrusted_remote_data": true, "data": response})
}
