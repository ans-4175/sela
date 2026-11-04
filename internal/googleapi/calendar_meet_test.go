package googleapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ans-4175/sela/internal/actions"
	"github.com/ans-4175/sela/internal/plans"
	"google.golang.org/api/calendar/v3"
)

func meetFixture() actions.Calendar {
	return actions.Calendar{CalendarID: "primary", SendUpdates: "all", Conference: &actions.Conference{Type: "google_meet", RequestID: strings.Repeat("a", 32)}, Event: &actions.Event{Summary: "Fixture", Start: actions.EventTime{DateTime: "2026-12-01T10:00:00+07:00", TimeZone: "Asia/Jakarta"}, End: actions.EventTime{DateTime: "2026-12-01T11:00:00+07:00", TimeZone: "Asia/Jakarta"}, Attendees: []string{"guest@example.test"}}}
}
func conferenceReply(status string) *calendar.Event {
	return &calendar.Event{Id: "fixture", Etag: "read-etag", ConferenceData: &calendar.ConferenceData{CreateRequest: &calendar.CreateConferenceRequest{RequestId: strings.Repeat("a", 32), Status: &calendar.ConferenceRequestStatus{StatusCode: status}}, ConferenceSolution: &calendar.ConferenceSolution{Key: &calendar.ConferenceSolutionKey{Type: "hangoutsMeet"}}, EntryPoints: []*calendar.EntryPoint{{EntryPointType: "video", Uri: "https://meet.google.com/abc-defg-hij"}}}}
}

func TestMeetContractAndPartialSuccessReceipt(t *testing.T) {
	for _, tc := range []struct {
		name, status string
		httpStatus   int
		change       func(*calendar.Event)
	}{
		{name: "ready", status: "success", httpStatus: 200},
		{name: "pending", status: "pending", httpStatus: 200},
		{name: "failure", status: "failure", httpStatus: 200},
		{name: "read error", status: "unverified", httpStatus: 503},
		{name: "wrong event", status: "unverified", httpStatus: 200, change: func(e *calendar.Event) { e.Id = "other" }},
		{name: "wrong request", status: "unverified", httpStatus: 200, change: func(e *calendar.Event) { e.ConferenceData.CreateRequest.RequestId = strings.Repeat("b", 32) }},
		{name: "unsafe link", status: "unverified", httpStatus: 200, change: func(e *calendar.Event) {
			e.ConferenceData.EntryPoints[0].Uri = "https://meet.google.com.evil.test/abc-defg-hij"
		}},
		{name: "missing conference", status: "unverified", httpStatus: 200, change: func(e *calendar.Event) { e.ConferenceData = nil }},
		{name: "conflicting request type", status: "unverified", httpStatus: 200, change: func(e *calendar.Event) {
			e.ConferenceData.CreateRequest.ConferenceSolutionKey = &calendar.ConferenceSolutionKey{Type: "addOn"}
		}},
		{name: "cancelled event", status: "unverified", httpStatus: 200, change: func(e *calendar.Event) { e.Status = "cancelled" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp := meetFixture()
			calls, posts := 0, 0
			c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Method == "POST" {
					posts++
					if calls != 1 || r.URL.Path != "/calendar/v3/calendars/primary/events" || r.URL.Query().Get("conferenceDataVersion") != "1" || r.URL.Query().Get("sendUpdates") != "all" {
						t.Error("conference/notification request mismatch")
					}
					var body calendar.Event
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					d := body.ConferenceData
					if d == nil || d.CreateRequest == nil || d.CreateRequest.RequestId != cp.Conference.RequestID || d.CreateRequest.ConferenceSolutionKey.Type != "hangoutsMeet" || len(d.EntryPoints) != 0 || d.ConferenceId != "" || d.CreateRequest.Status != nil || body.Summary != "Fixture" || len(body.Attendees) != 1 || body.Attendees[0].Email != "guest@example.test" {
						t.Error("approved conference/event changed")
					}
					return reply(200, `{"id":"fixture","etag":"insert-etag","conferenceData":{"createRequest":{"status":{"statusCode":"pending"}}}}`), nil
				}
				if calls != 2 || r.Method != "GET" || r.URL.Path != "/calendar/v3/calendars/primary/events/fixture" {
					t.Error("unexpected retry or reconciliation target")
				}
				event := conferenceReply("success")
				if tc.status == "pending" || tc.status == "failure" {
					event.ConferenceData.CreateRequest.Status.StatusCode = tc.status
				}
				if tc.change != nil {
					tc.change(event)
				}
				b, _ := json.Marshal(event)
				return reply(tc.httpStatus, string(b)), nil
			})})
			data, _ := json.Marshal(cp)
			raw, err := c.Perform(context.Background(), plans.Plan{Scope: plans.Scope{Action: "calendar.create", Payload: data}})
			if err != nil {
				t.Fatal("known event receipt lost", err)
			}
			var result meetCreateResult
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			ready := tc.name == "ready"
			if calls != 2 || posts != 1 || !result.EventCreated || result.EventID != "fixture" || result.ConferenceStatus != tc.status || result.MeetReady != ready || result.WorkflowComplete != ready || (result.MeetURL != "") != ready {
				t.Fatal("incorrect result or retry", string(raw), calls, posts)
			}
		})
	}
}

func TestPlainCreateAndUpdateNeverChangeConference(t *testing.T) {
	for _, kind := range []string{"calendar.create", "calendar.update"} {
		t.Run(kind, func(t *testing.T) {
			cp := meetFixture()
			cp.Conference = nil
			if kind == "calendar.update" {
				cp.EventID = "fixture"
				cp.ETag = "approved-etag"
				cp.Before = cp.Event
			}
			calls := 0
			c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.URL.Query().Has("conferenceDataVersion") {
					t.Error("ordinary write enabled conference changes")
				}
				var body map[string]json.RawMessage
				json.NewDecoder(r.Body).Decode(&body)
				if _, ok := body["conferenceData"]; ok {
					t.Error("ordinary write touched conference")
				}
				if kind == "calendar.update" && (r.Method != "PATCH" || r.Header.Get("If-Match") != cp.ETag) {
					t.Error("update guards changed")
				}
				return reply(200, `{"id":"fixture","etag":"etag"}`), nil
			})})
			payload, _ := json.Marshal(cp)
			r, err := c.Perform(context.Background(), plans.Plan{Scope: plans.Scope{Action: kind, Payload: payload}})
			var result map[string]any
			json.Unmarshal(r, &result)
			if err != nil || calls != 1 || len(result) != 2 || result["event_id"] != "fixture" {
				t.Fatal("legacy result/retry changed", err, string(r))
			}
		})
	}
}

func TestMeetInsertFailureNeverRetriesAndURLIsGoogleMeetOnly(t *testing.T) {
	for _, body := range []string{`{"error":{"code":503,"message":"SYNTHETIC_PRIVATE"}}`, `{}`} {
		calls := 0
		c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			calls++
			code := 200
			if strings.Contains(body, "error") {
				code = 503
			}
			return reply(code, body), nil
		})})
		payload, _ := json.Marshal(meetFixture())
		_, err := c.Perform(context.Background(), plans.Plan{Scope: plans.Scope{Action: "calendar.create", Payload: payload}})
		if err == nil || calls != 1 || strings.Contains(err.Error(), "PRIVATE") {
			t.Fatal("ambiguous insert retried or raw error leaked", err, calls)
		}
	}
	for _, uri := range []string{"http://meet.google.com/abc-defg-hij", "https://meet.google.com.evil.test/abc-defg-hij", "https://user@meet.google.com/abc-defg-hij", "https://meet.google.com:443/abc-defg-hij", "https://meet.google.com/abc-defg-hij?token=x", "https://meet.google.com/"} {
		if validMeetURL(uri) {
			t.Fatal("unsafe Meet URL accepted", uri)
		}
	}
	if !validMeetURL("https://meet.google.com/abc-defg-hij") {
		t.Fatal("expected Meet URL rejected")
	}
}
