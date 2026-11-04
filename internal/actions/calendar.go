package actions

import (
	"errors"
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
type Calendar struct {
	CalendarID  string `json:"calendar_id"`
	EventID     string `json:"event_id,omitempty"`
	ETag        string `json:"expected_etag,omitempty"`
	SendUpdates string `json:"send_updates"`
	Event       *Event `json:"event,omitempty"`
	Before      *Event `json:"before,omitempty"`
}

// Validate checks an executable snapshot, including the server-derived change precondition.
func (c Calendar) Validate(action string) error {
	if err := c.ValidateInput(action); err != nil {
		return err
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
