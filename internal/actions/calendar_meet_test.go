package actions

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConferenceIsCreateOnlyAndBoundToGeneratedRequest(t *testing.T) {
	cp := Calendar{CalendarID: "primary", SendUpdates: "all", Event: &Event{Summary: "Fixture", Start: EventTime{"2026-12-01T10:00:00+07:00", "Asia/Jakarta"}, End: EventTime{"2026-12-01T11:00:00+07:00", "Asia/Jakarta"}}}
	legacy, _ := json.Marshal(cp)
	if strings.Contains(string(legacy), "conference") {
		t.Fatal("legacy snapshot changed")
	}
	cp.Conference = &Conference{Type: "google_meet"}
	if cp.ValidateInput("calendar.create") != nil || cp.Validate("calendar.create") == nil {
		t.Fatal("preview/executable request ID distinction lost")
	}
	cp.Conference.RequestID = strings.Repeat("a", 32)
	if err := cp.Validate("calendar.create"); err != nil {
		t.Fatal(err)
	}
	cp.EventID = "fixture"
	cp.ETag = "etag"
	cp.Before = cp.Event
	if cp.ValidateInput("calendar.update") == nil || cp.ValidateInput("calendar.delete") == nil {
		t.Fatal("conference mutation beyond create allowed")
	}
	cp.EventID = ""
	cp.ETag = ""
	cp.Conference.Type = "arbitrary"
	if cp.ValidateInput("calendar.create") == nil {
		t.Fatal("arbitrary provider allowed")
	}
	cp.Conference.Type = "google_meet"
	cp.Conference.RequestID = "a query or existing Meet URL"
	if cp.ValidateInput("calendar.create") == nil {
		t.Fatal("invalid request ID allowed")
	}
}
