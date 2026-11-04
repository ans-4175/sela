package googleapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ans-4175/sela/internal/actions"
	"github.com/ans-4175/sela/internal/plans"
)

func TestMarkReadPreviewFixedBatchAndVerification(t *testing.T) {
	posts := 0
	changed := false
	client, err := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		if r.Method == "POST" {
			posts++
			if !strings.HasSuffix(r.URL.Path, "/messages/batchModify") {
				t.Error("unexpected mutation endpoint")
			}
			data, _ := io.ReadAll(r.Body)
			var body map[string]any
			json.Unmarshal(data, &body)
			if len(body) != 2 || len(body["ids"].([]any)) != 1 || body["ids"].([]any)[0] != "alpha" || len(body["removeLabelIds"].([]any)) != 1 || body["removeLabelIds"].([]any)[0] != "UNREAD" {
				t.Error("mutation widened approved scope", string(data))
			}
			changed = true
			return reply(204, ""), nil
		}
		if r.Method != "GET" || r.URL.Query().Get("format") != "metadata" {
			t.Error("preview/verification read more than metadata")
		}
		id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
		labels := []string{"INBOX"}
		if id == "alpha" && !changed {
			labels = append(labels, "UNREAD")
		}
		data, _ := json.Marshal(map[string]any{"id": id, "labelIds": labels, "payload": map[string]any{"headers": []map[string]string{{"name": "SUBJECT", "value": "Fixture subject"}, {"name": "From", "value": "sender@example.test"}}}})
		return reply(200, string(data)), nil
	})})
	if err != nil {
		t.Fatal(err)
	}
	payload, skipped, err := client.MarkReadPreview(context.Background(), []string{"alpha", "beta"})
	if err != nil || len(payload.Messages) != 1 || payload.Messages[0].ID != "alpha" || payload.Messages[0].Subject != "Fixture subject" || len(skipped) != 1 || skipped[0] != "beta" {
		t.Fatal("incorrect selected subset", payload, skipped, err)
	}
	if posts != 0 {
		t.Fatal("preview mutated mail")
	}
	data, _ := json.Marshal(payload)
	result, err := client.Perform(context.Background(), plans.Plan{Scope: plans.Scope{Action: "mail.mark-read", Payload: data}})
	if err != nil || posts != 1 || !strings.Contains(string(result), `"verified_read":true`) {
		t.Fatal("mark/read-back failed", posts, err)
	}
}

func TestMarkReadFailureDoesNotRetryOrPretendVerified(t *testing.T) {
	for _, writeFails := range []bool{true, false} {
		posts := 0
		client, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			if r.Method == "POST" {
				posts++
				if writeFails {
					return reply(503, `{"error":{"code":503,"message":"SYNTHETIC_PRIVATE_RESPONSE"}}`), nil
				}
				return reply(204, ""), nil
			}
			return reply(200, `{"id":"alpha","labelIds":["INBOX","UNREAD"]}`), nil
		})})
		data, _ := json.Marshal(actions.MarkRead{Messages: []actions.MarkReadMessage{{ID: "alpha"}}})
		result, err := client.Perform(context.Background(), plans.Plan{Scope: plans.Scope{Action: "mail.mark-read", Payload: data}})
		if err == nil || result != nil || posts != 1 || strings.Contains(err.Error(), "PRIVATE") {
			t.Fatal("uncertainty/retry mishandled", posts, err)
		}
	}
}
