package googleapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"regexp"

	"github.com/ans-4175/sela/internal/actions"
	"google.golang.org/api/calendar/v3"
)

func (c *Clients) createEvent(ctx context.Context, cp actions.Calendar) (json.RawMessage, error) {
	body := event(cp.Event)
	if cp.Conference != nil {
		body.ConferenceData = &calendar.ConferenceData{CreateRequest: &calendar.CreateConferenceRequest{RequestId: cp.Conference.RequestID, ConferenceSolutionKey: &calendar.ConferenceSolutionKey{Type: "hangoutsMeet"}}}
	}
	call := c.calendar.Events.Insert(cp.CalendarID, body).SendUpdates(cp.SendUpdates)
	if cp.Conference != nil {
		call.ConferenceDataVersion(1)
	}
	created, err := call.Context(ctx).Do()
	if err != nil {
		return nil, safe(err)
	}
	if created.Id == "" {
		return nil, errors.New("event creation response has no event ID; do not retry")
	}
	if cp.Conference == nil {
		return json.Marshal(map[string]string{"event_id": created.Id, "etag": created.Etag})
	}

	// Insertion is known to have succeeded. Retain its receipt even if the subsequent
	// read cannot prove that asynchronous conference generation has completed.
	result := meetCreateResult{EventID: created.Id, ETag: created.Etag, EventCreated: true, ConferenceStatus: "unverified", Notice: "Event created; invitations may already be sent. Meet is not verified ready. Read this event with calendar get; do not create a replacement or retry the insert."}
	current, err := c.Event(ctx, cp.CalendarID, created.Id)
	if err != nil || current.Id != created.Id || current.Status == "cancelled" {
		return json.Marshal(result)
	}
	result.ETag = current.Etag
	data := current.ConferenceData
	if data == nil || data.CreateRequest == nil || data.CreateRequest.RequestId != cp.Conference.RequestID || data.CreateRequest.Status == nil {
		return json.Marshal(result)
	}
	if key := data.CreateRequest.ConferenceSolutionKey; key != nil && key.Type != "hangoutsMeet" {
		return json.Marshal(result)
	}
	switch data.CreateRequest.Status.StatusCode {
	case "pending", "failure":
		result.ConferenceStatus = data.CreateRequest.Status.StatusCode
	case "success":
		if data.ConferenceSolution == nil || data.ConferenceSolution.Key == nil || data.ConferenceSolution.Key.Type != "hangoutsMeet" {
			break
		}
		for _, entry := range data.EntryPoints {
			if entry != nil && entry.EntryPointType == "video" && validMeetURL(entry.Uri) {
				result.ConferenceStatus = "success"
				result.MeetReady = true
				result.WorkflowComplete = true
				result.MeetURL = entry.Uri
				result.Notice = "Event created and Google Meet verified by read-back. Guest notification was requested according to send_updates; delivery/attendance are not verified."
				break
			}
		}
	}
	return json.Marshal(result)
}

type meetCreateResult struct {
	EventID          string `json:"event_id"`
	ETag             string `json:"etag"`
	EventCreated     bool   `json:"event_created"`
	ConferenceStatus string `json:"conference_status"`
	MeetReady        bool   `json:"meet_ready"`
	WorkflowComplete bool   `json:"workflow_complete"`
	MeetURL          string `json:"meet_url,omitempty"`
	Notice           string `json:"notice"`
}

var meetPath = regexp.MustCompile(`^/[a-z]{3}-[a-z]{4}-[a-z]{3}$`)

func validMeetURL(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Host == "meet.google.com" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && u.RawPath == "" && meetPath.MatchString(u.Path)
}
