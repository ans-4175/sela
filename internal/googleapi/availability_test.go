package googleapi

import (
	"context"
	"encoding/json"
	"google.golang.org/api/calendar/v3"
	"net/http"
	"testing"
)

func availabilityFixture() AvailabilityInput {
	return AvailabilityInput{ReadWindow: ReadWindow{Calendars: []string{"primary", "second"}, Start: "2026-12-01T09:00:00+07:00", End: "2026-12-01T17:00:00+07:00", TimeZone: "Asia/Jakarta"}, DurationMinutes: 60}
}
func freeBusyFixture() *calendar.FreeBusyResponse {
	return &calendar.FreeBusyResponse{TimeMin: "2026-12-01T02:00:00Z", TimeMax: "2026-12-01T10:00:00Z", Calendars: map[string]calendar.FreeBusyCalendar{
		"primary": {Busy: []*calendar.TimePeriod{{Start: "2026-12-01T08:00:00+07:00", End: "2026-12-01T10:00:00+07:00"}, {Start: "2026-12-01T10:00:00+07:00", End: "2026-12-01T11:00:00+07:00"}}},
		"second":  {Busy: []*calendar.TimePeriod{{Start: "2026-12-01T10:30:00+07:00", End: "2026-12-01T12:00:00+07:00"}, {Start: "2026-12-01T13:00:00+07:00", End: "2026-12-01T14:00:00+07:00"}}},
	}}
}
func TestAvailabilityUnionsClipsAndFindsMaximalWindows(t *testing.T) {
	out, err := availabilityResult(availabilityFixture(), freeBusyFixture())
	if err != nil || !out.Complete || len(out.Free) != 2 || len(out.Busy) != 2 {
		t.Fatal(out, err)
	}
	if out.Free[0].Start != "2026-12-01T12:00:00+07:00" || out.Free[0].End != "2026-12-01T13:00:00+07:00" || out.Free[1].Start != "2026-12-01T14:00:00+07:00" {
		t.Fatal(out)
	}
	r := freeBusyFixture()
	r.Calendars["primary"] = calendar.FreeBusyCalendar{Busy: []*calendar.TimePeriod{}}
	r.Calendars["second"] = calendar.FreeBusyCalendar{Busy: []*calendar.TimePeriod{}}
	out, err = availabilityResult(availabilityFixture(), r)
	if err != nil || len(out.Free) != 1 {
		t.Fatal(out, err)
	}
}
func TestAvailabilityNeverClaimsFreeOnUnknownOrMalformedCoverage(t *testing.T) {
	for _, mutate := range []func(*calendar.FreeBusyResponse){
		func(r *calendar.FreeBusyResponse) { delete(r.Calendars, "second") },
		func(r *calendar.FreeBusyResponse) {
			r.Calendars["second"] = calendar.FreeBusyCalendar{Errors: []*calendar.Error{{Reason: "notFound"}}}
		},
		func(r *calendar.FreeBusyResponse) { r.Calendars["second"] = calendar.FreeBusyCalendar{} },
		func(r *calendar.FreeBusyResponse) { r.TimeMax = "2026-12-02T00:00:00Z" },
		func(r *calendar.FreeBusyResponse) {
			r.Groups = map[string]calendar.FreeBusyGroup{"group": {Calendars: []string{"primary"}}}
		},
		func(r *calendar.FreeBusyResponse) {
			r.Calendars["second"] = calendar.FreeBusyCalendar{Busy: []*calendar.TimePeriod{{Start: "bad", End: "bad"}}}
		},
		func(r *calendar.FreeBusyResponse) {
			r.Calendars["second"] = calendar.FreeBusyCalendar{Busy: []*calendar.TimePeriod{nil}}
		},
	} {
		r := freeBusyFixture()
		mutate(r)
		out, err := availabilityResult(availabilityFixture(), r)
		if err == nil || out.Complete || len(out.Free) > 0 {
			t.Fatal("unsafe free claim", out, err)
		}
	}
}
func TestAvailabilityUsesOnlyExactReadOnlyFreeBusyRequest(t *testing.T) {
	calls := 0
	c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/calendar/v3/freeBusy" {
			t.Error(r.Method, r.URL.Path)
		}
		var in calendar.FreeBusyRequest
		json.NewDecoder(r.Body).Decode(&in)
		if len(in.Items) != 2 || in.Items[0].Id != "primary" || in.Items[1].Id != "second" || in.TimeZone != "Asia/Jakarta" || in.CalendarExpansionMax != 10 {
			t.Error(in)
		}
		b, _ := json.Marshal(freeBusyFixture())
		return reply(200, string(b)), nil
	})})
	out, err := c.Availability(context.Background(), availabilityFixture())
	if err != nil || !out.Complete || calls != 1 {
		t.Fatal(out, err, calls)
	}
}
func TestReadWindowOffsetDSTAndBounds(t *testing.T) {
	for _, mutate := range []func(*AvailabilityInput){
		func(in *AvailabilityInput) { in.TimeZone = "Local" }, func(in *AvailabilityInput) { in.Start = "2026-12-01T09:00:00Z" }, func(in *AvailabilityInput) { in.Calendars = []string{"primary", "primary"} }, func(in *AvailabilityInput) { in.DurationMinutes = 0 }, func(in *AvailabilityInput) { in.End = "2027-01-01T17:00:00+07:00" },
	} {
		in := availabilityFixture()
		mutate(&in)
		if in.Validate() == nil {
			t.Fatal(in)
		}
	}
	in := AvailabilityInput{ReadWindow: ReadWindow{Calendars: []string{"primary"}, Start: "2026-03-08T01:30:00-05:00", End: "2026-03-08T03:30:00-04:00", TimeZone: "America/New_York"}, DurationMinutes: 60}
	if err := in.Validate(); err != nil {
		t.Fatal("valid DST transition", err)
	}
	in.End = "2026-03-08T03:30:00-05:00"
	if in.Validate() == nil {
		t.Fatal("mismatched DST accepted")
	}
}
