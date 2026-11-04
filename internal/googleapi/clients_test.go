package googleapi

import (
	"context"
	"encoding/json"
	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/actions"
	"github.com/ans-4175/sela/internal/plans"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundtrip func(*http.Request) (*http.Response, error)

func (f roundtrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func reply(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}
func TestExactSendOnceAndSafeErrors(t *testing.T) {
	a := accounts.Account{Name: "personal", Email: "person@example.test"}
	m, e := actions.BuildMail(a, actions.MailInput{To: []string{"recipient@example.test"}, Subject: "Fixture", Body: "exact body"})
	if e != nil {
		t.Fatal(e)
	}
	data, _ := json.Marshal(m)
	p := plans.Plan{Scope: plans.Scope{Account: a, Action: "mail.send", Payload: data}}
	calls := 0
	c, e := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "POST" || !strings.HasSuffix(r.URL.Path, "/users/me/messages/send") {
			t.Error("unexpected SDK request", r.Method, r.URL.Path)
		}
		var got struct {
			Raw string `json:"raw"`
		}
		json.NewDecoder(r.Body).Decode(&got)
		if got.Raw != m.Raw {
			t.Error("approved bytes changed")
		}
		return reply(503, `{"error":{"code":503,"message":"SYNTHETIC_PRIVATE_RESPONSE"}}`), nil
	})})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = c.Perform(context.Background(), p); e == nil || strings.Contains(e.Error(), "PRIVATE") {
		t.Fatal("raw error exposed")
	}
	if calls != 1 {
		t.Fatal("automatic retry", calls)
	}
	if _, e = NewWithHTTP(context.Background(), nil); e == nil {
		t.Fatal("ambient client allowed")
	}
	if _, e = New(context.Background(), nil); e == nil {
		t.Fatal("implicit token source")
	}
}
func TestCalendarETagNotificationAndDelete(t *testing.T) {
	cp := actions.Calendar{CalendarID: "primary", EventID: "fixture", ETag: `"approved-etag"`, SendUpdates: "all", Before: &actions.Event{Summary: "old", Attendees: []string{"old@example.test"}}, Event: &actions.Event{Summary: "new", Start: actions.EventTime{DateTime: "2026-12-01T10:00:00+07:00", TimeZone: "Asia/Jakarta"}, End: actions.EventTime{DateTime: "2026-12-01T11:00:00+07:00", TimeZone: "Asia/Jakarta"}, Attendees: []string{"new@example.test"}}}
	calls := 0
	c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Header.Get("If-Match") != cp.ETag || r.URL.Query().Get("sendUpdates") != "all" {
			t.Error("ETag/notification not bound")
		}
		if calls == 1 {
			if r.Method != "PATCH" {
				t.Error("method")
			}
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			if body["summary"] != "new" {
				t.Error("payload mismatch")
			}
			return reply(412, `{"error":{"code":412,"message":"changed"}}`), nil
		}
		if r.Method != "DELETE" {
			t.Error("method")
		}
		return reply(204, ""), nil
	})})
	data, _ := json.Marshal(cp)
	if _, e := c.Perform(context.Background(), plans.Plan{Scope: plans.Scope{Action: "calendar.update", Payload: data}}); e == nil {
		t.Fatal("stale event accepted")
	}
	cp.Event = nil
	data, _ = json.Marshal(cp)
	if _, e := c.Perform(context.Background(), plans.Plan{Scope: plans.Scope{Action: "calendar.delete", Payload: data}}); e != nil {
		t.Fatal(e)
	}
	if calls != 2 {
		t.Fatal("retry")
	}
}
func TestIdentityIsolationAndBoundedReads(t *testing.T) {
	calls := 0
	c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return reply(200, `{"email":"other@example.test","verified_email":true}`), nil
	})})
	if c.Verify(context.Background(), accounts.Account{Name: "personal", Email: "person@example.test"}) == nil {
		t.Fatal("wrong account accepted")
	}
	before := calls
	if _, e := c.Search(context.Background(), "", 20, ""); e == nil {
		t.Fatal("unbounded search")
	}
	if _, e := c.Events(context.Background(), "", "bad", "bad", 200, ""); e == nil {
		t.Fatal("invalid calendar range")
	}
	if calls != before {
		t.Fatal("invalid reads reached transport")
	}
}
