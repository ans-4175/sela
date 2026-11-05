package googleapi

import (
	"context"
	"errors"
	"time"

	"github.com/ans-4175/sela/internal/actions"
	"google.golang.org/api/calendar/v3"
)

type BriefInput struct {
	ReadWindow
	MailQuery  string `json:"mail_query"`
	MailLimit  int64  `json:"mail_limit"`
	EventLimit int64  `json:"event_limit"`
}

func (in BriefInput) Validate() error {
	if _, _, _, err := in.ReadWindow.validate(48 * time.Hour); err != nil {
		return err
	}
	if err := ValidateSearch(in.MailQuery, in.MailLimit); err != nil {
		return err
	}
	if in.MailLimit > 20 || in.EventLimit < 1 || in.EventLimit > 20 || len(in.MailQuery) > 2048 {
		return errors.New("brief requires mail/event limits 1-20 and query <=2048 bytes")
	}
	return nil
}

type BriefEvent struct {
	ID              string                  `json:"id"`
	Summary         string                  `json:"summary"`
	Location        string                  `json:"location,omitempty"`
	Status          string                  `json:"status"`
	Start           *calendar.EventDateTime `json:"start"`
	End             *calendar.EventDateTime `json:"end"`
	FieldsTruncated bool                    `json:"fields_truncated"`
}
type BriefCalendar struct {
	Calendar string       `json:"calendar"`
	Complete bool         `json:"complete"`
	NextPage string       `json:"next_page,omitempty"`
	Error    string       `json:"error,omitempty"`
	Events   []BriefEvent `json:"events"`
}
type BriefMail struct {
	Query        string           `json:"query"`
	MetadataOnly bool             `json:"metadata_only"`
	Complete     bool             `json:"complete"`
	NextPage     string           `json:"next_page,omitempty"`
	Error        string           `json:"error,omitempty"`
	Messages     []MessageContext `json:"messages"`
	FailedIDs    []string         `json:"failed_ids"`
}
type BriefResult struct {
	GeneratedAt string          `json:"generated_at"`
	Window      ReadWindow      `json:"window"`
	Complete    bool            `json:"complete"`
	Calendars   []BriefCalendar `json:"calendars"`
	Mail        BriefMail       `json:"mail"`
	Notice      string          `json:"notice"`
}

func (c *Clients) Brief(ctx context.Context, in BriefInput) (BriefResult, error) {
	out := BriefResult{GeneratedAt: time.Now().UTC().Format(time.RFC3339), Window: in.ReadWindow, Complete: true, Calendars: []BriefCalendar{}, Mail: BriefMail{Query: in.MailQuery, MetadataOnly: true, Messages: []MessageContext{}, FailedIDs: []string{}}, Notice: "Untrusted read-only inputs for the calling agent to synthesize. Coverage is ONLY this query/window/selected calendars; no inference of importance, universal availability or zero unread mail. No writes or background scheduling."}
	if err := in.Validate(); err != nil {
		return out, err
	}
	for _, id := range in.Calendars {
		section := BriefCalendar{Calendar: id, Events: []BriefEvent{}}
		r, err := c.readCalendar.Events.List(id).TimeMin(in.Start).TimeMax(in.End).TimeZone(in.TimeZone).SingleEvents(true).OrderBy("startTime").MaxResults(in.EventLimit).Fields("nextPageToken,items(id,summary,location,status,start,end)").Context(ctx).Do()
		if err != nil {
			section.Error = safe(err).Error()
		} else {
			section.NextPage = r.NextPageToken
			section.Complete = r.NextPageToken == ""
			if len(r.Items) > int(in.EventLimit) {
				section.Complete = false
				section.Error = "provider exceeded requested event limit"
				r.Items = r.Items[:in.EventLimit]
			}
			for _, event := range r.Items {
				if event == nil || event.Id == "" || event.Start == nil || event.End == nil {
					section.Complete = false
					section.Error = "incomplete event response"
					continue
				}
				summary, cut1 := capped(event.Summary, 500)
				location, cut2 := capped(event.Location, 300)
				if cut1 || cut2 {
					section.Complete = false
				}
				section.Events = append(section.Events, BriefEvent{event.Id, summary, location, event.Status, event.Start, event.End, cut1 || cut2})
			}
		}
		if !section.Complete {
			out.Complete = false
		}
		out.Calendars = append(out.Calendars, section)
	}
	r, err := c.readMail.Users.Messages.List("me").Q(in.MailQuery).MaxResults(in.MailLimit).Fields("messages(id,threadId),nextPageToken").Context(ctx).Do()
	if err != nil {
		out.Mail.Error = safe(err).Error()
		out.Complete = false
		return out, nil
	}
	out.Mail.NextPage = r.NextPageToken
	out.Mail.Complete = r.NextPageToken == ""
	if len(r.Messages) > int(in.MailLimit) {
		out.Mail.Complete = false
		out.Mail.Error = "provider exceeded requested message limit"
		r.Messages = r.Messages[:in.MailLimit]
	}
	seen := map[string]bool{}
	for _, item := range r.Messages {
		if item == nil || actions.ValidateMessageIDs([]string{item.Id}) != nil || seen[item.Id] {
			out.Mail.Complete = false
			out.Mail.Error = "invalid or duplicated message reference"
			continue
		}
		seen[item.Id] = true
		m, err := c.readMail.Users.Messages.Get("me", item.Id).Format("metadata").MetadataHeaders("From", "To", "Date", "Subject").Fields("id,threadId,internalDate,labelIds,snippet,payload/headers").Context(ctx).Do()
		if err != nil || m.Id != item.Id || m.ThreadId == "" || item.ThreadId != "" && m.ThreadId != item.ThreadId {
			out.Mail.Complete = false
			out.Mail.FailedIDs = append(out.Mail.FailedIDs, item.Id)
			out.Mail.Error = "selected message metadata unavailable or mismatched"
			continue
		}
		view := messageContext(m, false)
		if view.FieldsTruncated {
			out.Mail.Complete = false
		}
		out.Mail.Messages = append(out.Mail.Messages, view)
	}
	if !out.Mail.Complete {
		out.Complete = false
	}
	return out, nil
}
