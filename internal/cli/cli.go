package cli

import (
	"context"
	"errors"
	"github.com/ans-4175/sela/internal/actions"
	"github.com/ans-4175/sela/internal/auth"
	"github.com/ans-4175/sela/internal/googleapi"
	"io"
	"strings"
)

func (a *App) Run(ctx context.Context, args []string) error {
	if a.Out == nil {
		a.Out = io.Discard
	}
	if len(args) == 0 || (len(args) == 1 && (args[0] == "help" || args[0] == "--help")) {
		_, e := io.WriteString(a.Out, help)
		return e
	}
	if args[0] == "--skill" {
		return skill(a.Out, args[1:])
	}
	if len(args) == 1 && args[0] == "version" {
		return encode(a.Out, map[string]string{"version": "0.1.0-dev", "stage": "offline-tested; live integration unverified"})
	}
	if len(args) < 2 {
		return errors.New("unknown command; use --help or --skill")
	}
	if args[0] == "account" {
		return a.accountCommand(ctx, args[1:])
	}
	if args[0] == "auth" && args[1] == "scopes" {
		if len(args) != 2 {
			return errors.New("auth scopes takes no arguments")
		}
		return encode(a.Out, map[string]any{"default": auth.Scopes(), "calendar_write_opt_in": auth.ProfileScopes(true), "granted": false})
	}
	alias, id, query, page, calendarID, start, end, file, action, profile, clientFile := "", "", "", "", "", "", "", "", "", "", ""
	limit := int64(20)
	reconsent := false
	allowed := map[string]*string{"account": &alias}
	switch args[0] + " " + args[1] {
	case "auth login":
		allowed["client"] = &clientFile
		allowed["profile"] = &profile
	case "auth status", "auth logout":
	case "mail search":
		allowed["query"] = &query
		allowed["page"] = &page
	case "mail get", "draft get":
		allowed["id"] = &id
	case "mail preview", "calendar preview":
		allowed["file"] = &file
		allowed["action"] = &action
	case "calendar list":
		allowed["page"] = &page
	case "calendar events":
		allowed["calendar"] = &calendarID
		allowed["start"] = &start
		allowed["end"] = &end
		allowed["page"] = &page
	case "calendar get":
		allowed["calendar"] = &calendarID
		allowed["id"] = &id
	case "plan show", "plan approve", "plan execute":
		allowed["id"] = &id
	default:
		return errors.New("unsupported command; no raw API/force/draft-send-by-ID/Calendar admin available")
	}
	numbers := map[string]*int64{}
	if args[1] == "search" || args[0] == "calendar" && (args[1] == "list" || args[1] == "events") {
		numbers["limit"] = &limit
	}
	booleans := map[string]*bool{}
	if args[0] == "auth" && args[1] == "login" {
		booleans["reauth"] = &reconsent
	}
	if e := parse(args[0]+" "+args[1], args[2:], allowed, numbers, booleans); e != nil {
		return e
	}
	// Reject invalid reads before opening credential storage or verifying Google identity.
	switch args[0] + " " + args[1] {
	case "mail search":
		if err := googleapi.ValidateSearch(query, limit); err != nil {
			return err
		}
	case "calendar events":
		if err := googleapi.ValidateEvents(calendarID, start, end, limit); err != nil {
			return err
		}
	case "calendar list":
		if limit < 1 || limit > 100 {
			return errors.New("limit 1-100 required")
		}
	case "mail get", "draft get", "calendar get":
		if strings.TrimSpace(id) == "" {
			return errors.New("explicit item ID required")
		}
		if args[0] == "calendar" && strings.TrimSpace(calendarID) == "" {
			return errors.New("explicit calendar ID required")
		}
	}
	account, e := a.account(alias)
	if e != nil {
		return e
	}
	switch args[0] {
	case "auth":
		return a.authCommand(ctx, args[1], account, clientFile, profile, reconsent)
	case "plan":
		return a.planCommand(ctx, args[1], account, id)
	case "mail":
		if args[1] == "preview" {
			if action != "send" && action != "draft-create" {
				return errors.New("mail preview --action send|draft-create required; draft update/send-by-ID unavailable")
			}
			var in actions.MailInput
			if e = readInput(file, &in); e != nil {
				return e
			}
			payload, e := actions.BuildMail(account, in)
			if e != nil {
				return e
			}
			kind := "mail.send"
			if action == "draft-create" {
				kind = "draft.create"
			}
			store, e := a.planStore()
			if e != nil {
				return e
			}
			p, e := store.Create(ctx, account, kind, payload)
			if e != nil {
				return e
			}
			return encode(a.Out, p)
		}
	case "calendar":
		if args[1] == "preview" {
			return a.calendarPreview(ctx, account, action, file)
		}
	}
	client, _, e := a.client(ctx, account)
	if e != nil {
		return e
	}
	var response any
	switch args[0] + " " + args[1] {
	case "mail search":
		response, e = client.Search(ctx, query, limit, page)
	case "mail get":
		response, e = client.Message(ctx, id)
	case "draft get":
		response, e = client.Draft(ctx, id)
	case "calendar list":
		response, e = client.Calendars(ctx, limit, page)
	case "calendar events":
		response, e = client.Events(ctx, calendarID, start, end, limit, page)
	case "calendar get":
		response, e = client.Event(ctx, calendarID, id)
	}
	if e != nil {
		return e
	}
	return encode(a.Out, map[string]any{"account": account, "untrusted_remote_data": true, "data": response})
}
