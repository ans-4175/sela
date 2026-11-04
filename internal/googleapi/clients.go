// Package googleapi exposes a narrow assistant-oriented surface over official Google SDKs.
package googleapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/actions"
	"github.com/ans-4175/sela/internal/plans"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/gmail/v1"
	googleerrors "google.golang.org/api/googleapi"
	identity "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
	"net/http"
	"strings"
	"time"
)

type Clients struct {
	mail     *gmail.Service
	calendar *calendar.Service
	identity *identity.Service
}

func New(ctx context.Context, source oauth2.TokenSource) (*Clients, error) {
	if source == nil {
		return nil, errors.New("explicit token source required")
	}
	client := &http.Client{Transport: &oauth2.Transport{Source: source, Base: http.DefaultTransport}, Timeout: 30 * time.Second}
	return NewWithHTTP(ctx, client)
}
func NewWithHTTP(ctx context.Context, c *http.Client) (*Clients, error) {
	if c == nil {
		return nil, errors.New("explicit HTTP client required; ADC disabled")
	}
	g, e := gmail.NewService(ctx, option.WithHTTPClient(c))
	if e != nil {
		return nil, errors.New("cannot initialize Gmail SDK")
	}
	cal, e := calendar.NewService(ctx, option.WithHTTPClient(c))
	if e != nil {
		return nil, errors.New("cannot initialize Calendar SDK")
	}
	id, e := identity.NewService(ctx, option.WithHTTPClient(c))
	if e != nil {
		return nil, errors.New("cannot initialize identity SDK")
	}
	return &Clients{g, cal, id}, nil
}
func safe(err error) error {
	if err == nil {
		return nil
	}
	var e *googleerrors.Error
	if errors.As(err, &e) {
		return fmt.Errorf("Google API rejected request (HTTP %d); response content withheld", e.Code)
	}
	return errors.New("Google API transport failed; response content withheld")
}
func (c *Clients) Verify(ctx context.Context, a accounts.Account) error {
	u, e := c.identity.Userinfo.Get().Context(ctx).Do()
	if e != nil {
		return safe(e)
	}
	if u.VerifiedEmail == nil || !*u.VerifiedEmail || !strings.EqualFold(u.Email, a.Email) {
		return errors.New("authenticated Google identity does not match explicit account")
	}
	return nil
}
func ValidateSearch(q string, limit int64) error {
	if strings.TrimSpace(q) == "" || limit < 1 || limit > 100 {
		return errors.New("explicit query and limit 1-100 required")
	}
	return nil
}
func (c *Clients) Search(ctx context.Context, q string, limit int64, page string) (any, error) {
	if err := ValidateSearch(q, limit); err != nil {
		return nil, err
	}
	r, e := c.mail.Users.Messages.List("me").Q(q).MaxResults(limit).PageToken(page).Context(ctx).Do()
	return r, safe(e)
}
func (c *Clients) Message(ctx context.Context, id string) (*gmail.Message, error) {
	if id == "" {
		return nil, errors.New("message ID required")
	}
	r, e := c.mail.Users.Messages.Get("me", id).Format("full").Context(ctx).Do()
	return r, safe(e)
}
func (c *Clients) Draft(ctx context.Context, id string) (any, error) {
	if id == "" {
		return nil, errors.New("draft ID required")
	}
	r, e := c.mail.Users.Drafts.Get("me", id).Format("full").Context(ctx).Do()
	return r, safe(e)
}
func (c *Clients) Calendars(ctx context.Context, limit int64, page string) (any, error) {
	if limit < 1 || limit > 100 {
		return nil, errors.New("limit 1-100 required")
	}
	r, e := c.calendar.CalendarList.List().MaxResults(limit).PageToken(page).Context(ctx).Do()
	return r, safe(e)
}
func ValidateEvents(id, start, end string, limit int64) error {
	s, e1 := time.Parse(time.RFC3339, start)
	e, e2 := time.Parse(time.RFC3339, end)
	if strings.TrimSpace(id) == "" || e1 != nil || e2 != nil || !s.Before(e) || e.Sub(s) > 366*24*time.Hour || limit < 1 || limit > 100 {
		return errors.New("explicit calendar, RFC3339 range <=366 days and limit 1-100 required")
	}
	return nil
}
func (c *Clients) Events(ctx context.Context, id, start, end string, limit int64, page string) (any, error) {
	if err := ValidateEvents(id, start, end, limit); err != nil {
		return nil, err
	}
	r, err := c.calendar.Events.List(id).TimeMin(start).TimeMax(end).SingleEvents(true).OrderBy("startTime").MaxResults(limit).PageToken(page).Context(ctx).Do()
	return r, safe(err)
}
func (c *Clients) Event(ctx context.Context, calendarID, id string) (*calendar.Event, error) {
	if calendarID == "" || id == "" {
		return nil, errors.New("calendar and event IDs required")
	}
	r, e := c.calendar.Events.Get(calendarID, id).Context(ctx).Do()
	return r, safe(e)
}
func event(e *actions.Event) *calendar.Event {
	attendees := []*calendar.EventAttendee{}
	for _, email := range e.Attendees {
		attendees = append(attendees, &calendar.EventAttendee{Email: email})
	}
	return &calendar.Event{Summary: e.Summary, Description: e.Description, Location: e.Location, Start: &calendar.EventDateTime{DateTime: e.Start.DateTime, TimeZone: e.Start.TimeZone}, End: &calendar.EventDateTime{DateTime: e.End.DateTime, TimeZone: e.End.TimeZone}, Attendees: attendees, ForceSendFields: []string{"Description", "Location", "Attendees"}}
}
func EventView(e *calendar.Event) *actions.Event {
	out := &actions.Event{Summary: e.Summary, Description: e.Description, Location: e.Location}
	if e.Start != nil {
		out.Start = actions.EventTime{DateTime: e.Start.DateTime, TimeZone: e.Start.TimeZone}
	}
	if e.End != nil {
		out.End = actions.EventTime{DateTime: e.End.DateTime, TimeZone: e.End.TimeZone}
	}
	for _, a := range e.Attendees {
		out.Attendees = append(out.Attendees, a.Email)
	}
	return out
}

// Perform sends the immutable snapshot once. It contains NO automatic retry loop.
func (c *Clients) Perform(ctx context.Context, p plans.Plan) (json.RawMessage, error) {
	if p.Scope.Action == "mail.mark-read" {
		var payload actions.MarkRead
		if json.Unmarshal(p.Scope.Payload, &payload) != nil {
			return nil, errors.New("invalid mark-read snapshot")
		}
		return c.markRead(ctx, payload)
	}
	if p.Scope.Action == "mail.send" || p.Scope.Action == "draft.create" {
		var m actions.Mail
		if json.Unmarshal(p.Scope.Payload, &m) != nil || m.Validate(p.Scope.Account) != nil {
			return nil, errors.New("invalid immutable mail snapshot")
		}
		message := &gmail.Message{Raw: m.Raw}
		if p.Scope.Action == "mail.send" {
			r, e := c.mail.Users.Messages.Send("me", message).Context(ctx).Do()
			if e != nil {
				return nil, safe(e)
			}
			return json.Marshal(map[string]string{"message_id": r.Id, "thread_id": r.ThreadId})
		}
		r, e := c.mail.Users.Drafts.Create("me", &gmail.Draft{Message: message}).Context(ctx).Do()
		if e != nil {
			return nil, safe(e)
		}
		return json.Marshal(map[string]string{"draft_id": r.Id})
	}
	var cp actions.Calendar
	if json.Unmarshal(p.Scope.Payload, &cp) != nil || cp.Validate(p.Scope.Action) != nil {
		return nil, errors.New("invalid calendar payload")
	}
	switch p.Scope.Action {
	case "calendar.create":
		r, e := c.calendar.Events.Insert(cp.CalendarID, event(cp.Event)).SendUpdates(cp.SendUpdates).Context(ctx).Do()
		if e != nil {
			return nil, safe(e)
		}
		return json.Marshal(map[string]string{"event_id": r.Id, "etag": r.Etag})
	case "calendar.update":
		call := c.calendar.Events.Patch(cp.CalendarID, cp.EventID, event(cp.Event)).SendUpdates(cp.SendUpdates).Context(ctx)
		call.Header().Set("If-Match", cp.ETag)
		r, e := call.Do()
		if e != nil {
			return nil, safe(e)
		}
		return json.Marshal(map[string]string{"event_id": r.Id, "etag": r.Etag})
	case "calendar.delete":
		call := c.calendar.Events.Delete(cp.CalendarID, cp.EventID).SendUpdates(cp.SendUpdates).Context(ctx)
		call.Header().Set("If-Match", cp.ETag)
		return json.RawMessage(`{"deleted":true}`), safe(call.Do())
	}
	return nil, errors.New("unsupported mutation")
}
