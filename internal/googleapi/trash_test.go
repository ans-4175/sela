package googleapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/ans-4175/sela/internal/actions"
	"github.com/ans-4175/sela/internal/plans"
)

func fixtureTrash(labels ...string) string {
	data, _ := json.Marshal(map[string]any{"id": "alpha", "threadId": "thread1", "internalDate": "1234", "labelIds": labels, "payload": map[string]any{"headers": []map[string]string{{"name": "From", "value": "sender@example.test"}, {"name": "Subject", "value": "Fixture"}, {"name": "Date", "value": "fixture date"}}}})
	return string(data)
}
func TestTrashExactPreviewFreshCheckAndReadback(t *testing.T) {
	writes, reads := 0, 0
	c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" {
			writes++
			if r.URL.Path != "/gmail/v1/users/me/messages/alpha/trash" {
				t.Errorf("unexpected endpoint %s", r.URL.Path)
			}
			return reply(200, `{"id":"alpha"}`), nil
		}
		if r.Method != "GET" || r.URL.Query().Get("format") != "metadata" {
			t.Fatal("unexpected request", r.Method, r.URL)
		}
		reads++
		if writes > 0 {
			return reply(200, `{"id":"alpha","labelIds":["TRASH"]}`), nil
		}
		return reply(200, fixtureTrash("INBOX", "CATEGORY_PROMOTIONS")), nil
	})})
	snapshot, err := c.TrashPreview(context.Background(), "alpha")
	if err != nil || writes != 0 {
		t.Fatal(snapshot, err)
	}
	data, _ := json.Marshal(snapshot)
	result, err := c.Perform(context.Background(), plans.Plan{Scope: plans.Scope{Action: "mail.trash", Payload: data}})
	if err != nil || writes != 1 || reads != 3 || !strings.Contains(string(result), "verified_trashed") {
		t.Fatal(writes, reads, string(result), err)
	}
}
func TestTrashProtectionAndStaleBeforeWrite(t *testing.T) {
	for _, label := range []string{"STARRED", "IMPORTANT", "TRASH", "SPAM", "DRAFT", "SENT"} {
		c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" {
				t.Fatal("write")
			}
			return reply(200, fixtureTrash("INBOX", label)), nil
		})})
		if _, err := c.TrashPreview(context.Background(), "alpha"); err == nil {
			t.Fatal("protected", label)
		}
	}
	for _, labels := range [][]string{{"CATEGORY_PROMOTIONS"}, {"INBOX", "STARRED"}, {"INBOX", "CATEGORY_SOCIAL"}, {"INBOX", "CATEGORY_PROMOTIONS", "UNREAD"}} {
		writes := 0
		c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			if r.Method == "POST" {
				writes++
			}
			return reply(200, fixtureTrash(labels...)), nil
		})})
		snap := actions.Trash{ID: "alpha", ThreadID: "thread1", InternalDate: 1234, From: "sender@example.test", Subject: "Fixture", Date: "fixture date", Labels: []string{"CATEGORY_PROMOTIONS", "INBOX"}}
		data, _ := json.Marshal(snap)
		_, err := c.Perform(context.Background(), plans.Plan{Scope: plans.Scope{Action: "mail.trash", Payload: data}})
		if err == nil || writes != 0 {
			t.Fatal("stale must not write", labels, writes, err)
		}
	}
}
func TestTrashUnknownNeverRetries(t *testing.T) {
	for _, mode := range []string{"transport", "http", "readback", "wrong-id"} {
		writes := 0
		c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			if r.Method == "POST" {
				writes++
				if mode == "transport" {
					return nil, errors.New("synthetic")
				}
				if mode == "http" {
					return reply(503, `{}`), nil
				}
				return reply(200, `{"id":"alpha"}`), nil
			}
			if writes > 0 {
				if mode == "wrong-id" {
					return reply(200, `{"id":"other","labelIds":["TRASH"]}`), nil
				}
				return reply(200, `{"id":"alpha","labelIds":["INBOX"]}`), nil
			}
			return reply(200, fixtureTrash("INBOX")), nil
		})})
		snap, _ := c.TrashPreview(context.Background(), "alpha")
		data, _ := json.Marshal(snap)
		_, err := c.Perform(context.Background(), plans.Plan{Scope: plans.Scope{Action: "mail.trash", Payload: data}})
		if err == nil || writes != 1 {
			t.Fatal(mode, writes, err)
		}
	}
}
func TestTrashInvalidMetadataRejected(t *testing.T) {
	for _, body := range []string{`{}`, fixtureTrash("INBOX", "INBOX"), strings.Replace(fixtureTrash("INBOX"), `"id":"alpha"`, `"id":"other"`, 1), strings.Replace(fixtureTrash("INBOX"), `"name":"Date"`, `"name":"NoDate"`, 1)} {
		c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) { return reply(200, body), nil })})
		if _, err := c.TrashPreview(context.Background(), "alpha"); err == nil {
			t.Fatal("invalid metadata accepted", body)
		}
	}
}
