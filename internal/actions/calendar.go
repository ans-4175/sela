package actions

import (
	"errors"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"
)

type EventTime struct {
	DateTime string `json:"date_time"`
	TimeZone string `json:"time_zone"`
}
type Event struct {
	Summary     string    `json:"summary"`
	Description string    `json:"description,omitempty"`
	Location    string    `json:"location,omitempty"`
	Start       EventTime `json:"start"`
	End         EventTime `json:"end"`
	Attendees   []string  `json:"attendees,omitempty"`
}

// MeetCreationNotice discloses partial-success risk before the agent records chat consent.
const MeetCreationNotice = "A new Google Meet is requested asynchronously. Event/guest invitations can exist before a link is ready; pending/failure leaves the event in place. Plan succeeded means event creation, not Meet readiness: require result.meet_ready and result.workflow_complete. No automatic retry, replacement event or cancellation; reconcile with calendar get."

// Conference requests a new Google Meet for one new event, never a shared/copied link.
type Conference struct {
	Type      string `json:"type"`
	RequestID string `json:"request_id,omitempty"`
}

var conferenceRequestID = regexp.MustCompile(`^[a-f0-9]{32}$`)

type Calendar struct {
	CalendarID  string      `json:"calendar_id"`
	EventID     string      `json:"event_id,omitempty"`
	ETag        string      `json:"expected_etag,omitempty"`
	SendUpdates string      `json:"send_updates"`
	Event       *Event      `json:"event,omitempty"`
	Before      *Event      `json:"before,omitempty"`
	Conference  *Conference `json:"conference,omitempty"`
}

// Validate checks an executable snapshot, including the server-derived change precondition.
func (c Calendar) Validate(action string) error {
	if err := c.ValidateInput(action); err != nil {
		return err
	}
	if c.Conference != nil && !conferenceRequestID.MatchString(c.Conference.RequestID) {
		return errors.New("Meet create requires the preview-generated conference request ID")
	}
	if action != "calendar.create" && (c.ETag == "" || c.Before == nil) {
		return errors.New("update/delete require current snapshot and exact ETag")
	}
	return nil
}

// ValidateInput rejects malformed user input before an update/delete preview reads Google.
func (c Calendar) ValidateInput(action string) error {
	if c.CalendarID == "" || (c.SendUpdates != "all" && c.SendUpdates != "none") {
		return errors.New("explicit calendar_id and send_updates all|none required")
	}
	if action != "calendar.create" && action != "calendar.update" && action != "calendar.delete" {
		return errors.New("unsupported calendar action")
	}
	if c.Conference != nil {
		if action != "calendar.create" || c.Conference.Type != "google_meet" {
			return errors.New("conference supports google_meet on create only; ordinary updates preserve conferences")
		}
		if c.Conference.RequestID != "" && !conferenceRequestID.MatchString(c.Conference.RequestID) {
			return errors.New("invalid conference request ID")
		}
	}
	if action == "calendar.create" && (c.EventID != "" || c.ETag != "") {
		return errors.New("create must not carry event ID/ETag")
	}
	if action != "calendar.create" && (strings.TrimSpace(c.EventID) == "" || strings.ContainsAny(c.ETag, "\r\n")) {
		return errors.New("update/delete require event ID and a valid optional ETag")
	}
	if action == "calendar.delete" {
		if c.Event != nil {
			return errors.New("delete must not include a replacement event")
		}
		return nil
	}
	if c.Event == nil || strings.TrimSpace(c.Event.Summary) == "" || len(c.Event.Summary) > 1024 || len(c.Event.Description) > 64000 || len(c.Event.Attendees) > 100 {
		return errors.New("invalid event summary/size/attendees")
	}
	s, e1 := time.Parse(time.RFC3339, c.Event.Start.DateTime)
	e, e2 := time.Parse(time.RFC3339, c.Event.End.DateTime)
	if e1 != nil || e2 != nil || !s.Before(e) || e.Sub(s) > 31*24*time.Hour {
		return errors.New("explicit RFC3339 event range <=31 days required")
	}
	for _, t := range []EventTime{c.Event.Start, c.Event.End} {
		if t.TimeZone == "" {
			return errors.New("explicit IANA event time zone required")
		}
		if t.TimeZone == "Local" {
			return errors.New("use an explicit IANA time zone, not machine-local time")
		}
		location, err := time.LoadLocation(t.TimeZone)
		if err != nil {
			return errors.New("invalid event time zone")
		}
		instant, _ := time.Parse(time.RFC3339, t.DateTime) // parsed above before this loop
		_, suppliedOffset := instant.Zone()
		_, zoneOffset := instant.In(location).Zone()
		if suppliedOffset != zoneOffset {
			return errors.New("event timestamp offset does not match its time zone on this date")
		}
	}
	for _, a := range c.Event.Attendees {
		if !ValidateAddress(a) {
			return errors.New("event attendees must be plain email addresses")
		}
	}
	return nil
}
