package googleapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func briefFixture() BriefInput {
	return BriefInput{ReadWindow: ReadWindow{Calendars: []string{"primary", "second"}, Start: "2026-12-01T00:00:00+07:00", End: "2026-12-02T00:00:00+07:00", TimeZone: "Asia/Jakarta"}, MailQuery: "newer_than:1d is:unread", MailLimit: 2, EventLimit: 3}
}
func TestBriefReadOnlyBoundedPartialPaginationAndMetadata(t *testing.T) {
	calls := 0
	c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" {
			t.Fatal("brief mutated", r.Method)
		}
		switch {
		case strings.Contains(r.URL.Path, "/calendars/primary/events"):
			if r.URL.Query().Get("maxResults") != "3" || r.URL.Query().Get("singleEvents") != "true" || r.URL.Query().Get("timeZone") != "Asia/Jakarta" {
				t.Fatal("agenda bounds")
			}
			return reply(200, `{"items":[{"id":"ev1","summary":"fixture","start":{"date":"2026-12-01"},"end":{"date":"2026-12-02"}}]}`), nil
		case strings.Contains(r.URL.Path, "/calendars/second/events"):
			return reply(403, `{"error":{"code":403,"message":"SYNTHETIC_PRIVATE_RESPONSE"}}`), nil
		case strings.HasSuffix(r.URL.Path, "/messages"):
			if r.URL.Query().Get("maxResults") != "2" || r.URL.Query().Get("q") != briefFixture().MailQuery {
				t.Fatal("mail bounds")
			}
			return reply(200, `{"messages":[{"id":"m1","threadId":"thread1"},{"id":"m2","threadId":"thread2"}],"nextPageToken":"fixture-next"}`), nil
		case strings.HasSuffix(r.URL.Path, "/messages/m1"):
			if r.URL.Query().Get("format") != "metadata" {
				t.Fatal("body harvest")
			}
			return reply(200, `{"id":"m1","threadId":"thread1","snippet":"fixture","payload":{"headers":[{"name":"Subject","value":"fixture"}]}}`), nil
		case strings.HasSuffix(r.URL.Path, "/messages/m2"):
			return reply(503, `{"error":{"code":503,"message":"SYNTHETIC_PRIVATE_RESPONSE"}}`), nil
		default:
			t.Fatal("unexpected API", r.URL.Path)
		}
		return nil, nil
	})})
	out, err := c.Brief(context.Background(), briefFixture())
	if err != nil || out.Complete || calls != 5 || len(out.Mail.Messages) != 1 || len(out.Mail.FailedIDs) != 1 || out.Mail.NextPage != "fixture-next" || out.Mail.Complete || !out.Mail.MetadataOnly || !out.Calendars[0].Complete || out.Calendars[1].Complete {
		t.Fatal(out, err, calls)
	}
	data, _ := json.Marshal(out)
	if strings.Contains(string(data), "SYNTHETIC_PRIVATE") {
		t.Fatal("error leaked")
	}
}
func TestBriefCompleteIsOnlySelectedCoverageAndInvalidNeverCalls(t *testing.T) {
	calls := 0
	c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if strings.Contains(r.URL.Path, "/events") {
			return reply(200, `{"items":[]}`), nil
		}
		return reply(200, `{"messages":[]}`), nil
	})})
	out, err := c.Brief(context.Background(), briefFixture())
	if err != nil || !out.Complete || calls != 3 {
		t.Fatal(out, err, calls)
	}
	in := briefFixture()
	in.MailLimit = 21
	if _, err := c.Brief(context.Background(), in); err == nil || calls != 3 {
		t.Fatal("invalid limit reached API")
	}
	in = briefFixture()
	in.End = "2026-12-04T00:00:00+07:00"
	if in.Validate() == nil {
		t.Fatal("brief window unbounded")
	}
}
