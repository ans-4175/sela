package googleapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func TestBatchPreviewSkipAndOneWritePerMessage(t *testing.T) {
	writes := 0
	c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" {
			writes++
			if !strings.HasSuffix(r.URL.Path, "/messages/alpha/trash") {
				t.Fatal("wrong mutation", r.URL.Path)
			}
			return reply(200, `{"id":"alpha"}`), nil
		}
		if writes > 0 {
			return reply(200, `{"id":"alpha","labelIds":["TRASH"]}`), nil
		}
		body := fixtureTrash("INBOX", "CATEGORY_PROMOTIONS")
		if strings.HasSuffix(r.URL.Path, "/beta") {
			body = strings.Replace(body, `"id":"alpha"`, `"id":"beta"`, 1)
			body = strings.Replace(body, `"INBOX"`, `"INBOX","STARRED"`, 1)
		}
		return reply(200, body), nil
	})})
	batch, skipped, err := c.TrashBatchPreview(context.Background(), []string{"alpha", "beta"}, "inbox", "CATEGORY_PROMOTIONS")
	if err != nil || len(batch.Messages) != 1 || len(skipped) != 1 || writes != 0 {
		t.Fatal(batch, skipped, err)
	}
	if _, err = c.TrashBatchItem(context.Background(), batch, 0); err != nil || writes != 1 {
		t.Fatal(err, writes)
	}
}
func TestBatchSpamAndProtected(t *testing.T) {
	for _, labels := range [][]string{{"SPAM"}, {"SPAM", "STARRED"}, {"SPAM", "IMPORTANT"}, {"SPAM", "DRAFT"}, {"SPAM", "SENT"}, {"SPAM", "TRASH"}, {"INBOX"}} {
		c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) { return reply(200, fixtureTrash(labels...)), nil })})
		b, skipped, err := c.TrashBatchPreview(context.Background(), []string{"alpha"}, "spam", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(labels) == 1 && labels[0] == "SPAM" {
			if len(b.Messages) != 1 || len(skipped) != 0 {
				t.Fatal(labels)
			}
		} else if len(b.Messages) != 0 || len(skipped) != 1 {
			t.Fatal(labels, b, skipped)
		}
	}
}
func TestBatchNoPlanOnReadErrorOrWrongCategory(t *testing.T) {
	c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		if strings.HasSuffix(r.URL.Path, "/beta") {
			return reply(503, `{}`), nil
		}
		return reply(200, fixtureTrash("INBOX", "CATEGORY_SOCIAL")), nil
	})})
	_, _, err := c.TrashBatchPreview(context.Background(), []string{"alpha", "beta"}, "inbox", "CATEGORY_PROMOTIONS")
	if err == nil {
		t.Fatal("partial read silently prepared")
	}
	b, skips, err := c.TrashBatchPreview(context.Background(), []string{"alpha"}, "inbox", "CATEGORY_PROMOTIONS")
	if err != nil || len(b.Messages) != 0 || len(skips) != 1 {
		t.Fatal(b, skips, err)
	}
}
