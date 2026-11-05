package googleapi

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"google.golang.org/api/calendar/v3"
)

type ReadWindow struct {
	Calendars []string `json:"calendars"`
	Start     string   `json:"start"`
	End       string   `json:"end"`
	TimeZone  string   `json:"time_zone"`
}

func (w ReadWindow) validate(max time.Duration) (time.Time, time.Time, *time.Location, error) {
	s, e1 := time.Parse(time.RFC3339, w.Start)
	e, e2 := time.Parse(time.RFC3339, w.End)
	zone, e3 := time.LoadLocation(w.TimeZone)
	if e1 != nil || e2 != nil || e3 != nil || w.TimeZone == "" || w.TimeZone == "Local" || !s.Before(e) || e.Sub(s) > max || len(w.Calendars) < 1 || len(w.Calendars) > 10 {
		return s, e, nil, errors.New("require 1-10 explicit calendars, matching IANA timezone/RFC3339 window within allowed range")
	}
	for _, v := range []time.Time{s, e} {
		_, got := v.Zone()
		_, want := v.In(zone).Zone()
		if got != want {
			return s, e, nil, errors.New("timestamp offset does not match selected timezone on that date")
		}
	}
	seen := map[string]bool{}
	for _, id := range w.Calendars {
		if strings.TrimSpace(id) != id || id == "" || len(id) > 320 || strings.ContainsAny(id, "\x00\r\n") || seen[id] {
			return s, e, nil, errors.New("calendar IDs must be explicit, unique and bounded")
		}
		seen[id] = true
	}
	return s, e, zone, nil
}

type AvailabilityInput struct {
	ReadWindow
	DurationMinutes int `json:"duration_minutes"`
}

func (in AvailabilityInput) Validate() error {
	_, _, _, err := in.ReadWindow.validate(14 * 24 * time.Hour)
	if err != nil {
		return err
	}
	if in.DurationMinutes < 1 || in.DurationMinutes > 1440 {
		return errors.New("duration_minutes must be 1-1440")
	}
	s, _ := time.Parse(time.RFC3339, in.Start)
	e, _ := time.Parse(time.RFC3339, in.End)
	if time.Duration(in.DurationMinutes)*time.Minute > e.Sub(s) {
		return errors.New("duration exceeds search window")
	}
	return nil
}

type TimeWindow struct {
	Start string `json:"start"`
	End   string `json:"end"`
}
type AvailabilityResult struct {
	Complete  bool         `json:"complete"`
	TimeZone  string       `json:"time_zone"`
	Calendars []string     `json:"calendars"`
	Free      []TimeWindow `json:"free_windows"`
	Busy      []TimeWindow `json:"busy_windows"`
	Notice    string       `json:"notice"`
}

func (c *Clients) Availability(ctx context.Context, in AvailabilityInput) (AvailabilityResult, error) {
	if err := in.Validate(); err != nil {
		return AvailabilityResult{}, err
	}
	items := make([]*calendar.FreeBusyRequestItem, 0, len(in.Calendars))
	for _, id := range in.Calendars {
		items = append(items, &calendar.FreeBusyRequestItem{Id: id})
	}
	response, err := c.readCalendar.Freebusy.Query(&calendar.FreeBusyRequest{TimeMin: in.Start, TimeMax: in.End, TimeZone: in.TimeZone, CalendarExpansionMax: 10, GroupExpansionMax: 1, Items: items}).Context(ctx).Do()
	if err != nil {
		return AvailabilityResult{}, safe(err)
	}
	return availabilityResult(in, response)
}

func availabilityResult(in AvailabilityInput, r *calendar.FreeBusyResponse) (AvailabilityResult, error) {
	out := AvailabilityResult{TimeZone: in.TimeZone, Calendars: in.Calendars, Free: []TimeWindow{}, Busy: []TimeWindow{}, Notice: "Free only within these selected calendars and explicit window; no inferred working hours, attendee availability or scheduling permission."}
	if err := in.Validate(); err != nil {
		return out, err
	}
	start, end, zone, _ := in.ReadWindow.validate(14 * 24 * time.Hour)
	fail := func() (AvailabilityResult, error) {
		return out, errors.New("free/busy incomplete or invalid; no free-slot claim is safe")
	}
	if r == nil || len(r.Groups) > 0 || len(r.Calendars) != len(in.Calendars) {
		return fail()
	}
	rs, e1 := time.Parse(time.RFC3339, r.TimeMin)
	re, e2 := time.Parse(time.RFC3339, r.TimeMax)
	if e1 != nil || e2 != nil || !rs.Equal(start) || !re.Equal(end) {
		return fail()
	}
	type interval struct{ s, e time.Time }
	busy := []interval{}
	for _, id := range in.Calendars {
		cal, ok := r.Calendars[id]
		if !ok || len(cal.Errors) > 0 || cal.Busy == nil {
			return fail()
		}
		for _, b := range cal.Busy {
			if b == nil {
				return fail()
			}
			s, e1 := time.Parse(time.RFC3339, b.Start)
			e, e2 := time.Parse(time.RFC3339, b.End)
			if e1 != nil || e2 != nil || !s.Before(e) {
				return fail()
			}
			if !s.Before(end) || !start.Before(e) {
				continue
			}
			if s.Before(start) {
				s = start
			}
			if e.After(end) {
				e = end
			}
			busy = append(busy, interval{s, e})
		}
	}
	sort.Slice(busy, func(i, j int) bool { return busy[i].s.Before(busy[j].s) })
	merged := []interval{}
	for _, b := range busy {
		if len(merged) > 0 && !merged[len(merged)-1].e.Before(b.s) {
			if b.e.After(merged[len(merged)-1].e) {
				merged[len(merged)-1].e = b.e
			}
		} else {
			merged = append(merged, b)
		}
	}
	view := func(s, e time.Time) TimeWindow {
		return TimeWindow{s.In(zone).Format(time.RFC3339), e.In(zone).Format(time.RFC3339)}
	}
	cursor := start
	min := time.Duration(in.DurationMinutes) * time.Minute
	for _, b := range merged {
		if b.s.Sub(cursor) >= min {
			out.Free = append(out.Free, view(cursor, b.s))
		}
		out.Busy = append(out.Busy, view(b.s, b.e))
		cursor = b.e
	}
	if end.Sub(cursor) >= min {
		out.Free = append(out.Free, view(cursor, end))
	}
	out.Complete = true
	return out, nil
}
