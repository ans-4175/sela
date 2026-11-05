package googleapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"google.golang.org/api/gmail/v1"
)

const syntheticThread = `{"id":"thread1","messages":[{"id":"m3","threadId":"thread1","internalDate":"300","snippet":"third"},{"id":"m1","threadId":"thread1","internalDate":"100","snippet":"first"},{"id":"m2","threadId":"thread1","internalDate":"200","snippet":"second"}]}`

func TestThreadReadsOnlySelectedBodiesInOrderWithoutMutation(t *testing.T) {
	calls := []string{}
	c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		calls = append(calls, r.URL.Path)
		if r.Method != "GET" {
			t.Fatal("mutation", r.Method)
		}
		if strings.HasSuffix(r.URL.Path, "/threads/thread1") {
			if r.URL.Query().Get("format") != "metadata" {
				t.Fatal("whole-thread body harvest")
			}
			return reply(200, syntheticThread), nil
		}
		id, date := "m2", "200"
		if strings.HasSuffix(r.URL.Path, "/messages/m3") {
			id, date = "m3", "300"
		} else if !strings.HasSuffix(r.URL.Path, "/messages/m2") {
			t.Fatal("unexpected body/attachment read", r.URL.Path)
		}
		if r.URL.Query().Get("format") != "full" {
			t.Fatal("body format")
		}
		return reply(200, fmt.Sprintf(`{"id":%q,"threadId":"thread1","internalDate":%q,"payload":{"mimeType":"text/plain","headers":[{"name":"Subject","value":"untrusted fixture"}],"body":{"data":%q,"size":5}}}`, id, date, base64.RawURLEncoding.EncodeToString([]byte("hello")))), nil
	})})
	out, err := c.Thread(context.Background(), "thread1", 2)
	if err != nil || out.Total != 3 || out.Omitted != 1 || out.Complete || out.Returned != 2 || len(calls) != 3 {
		t.Fatal(out, err, calls)
	}
	if out.Messages[0].ID != "m2" || out.Messages[1].ID != "m3" || out.Messages[0].TextExcerpt != "hello" || !out.Messages[0].ExcerptComplete {
		t.Fatal(out)
	}
}
func TestThreadPartialBodyAndOversizedMetadataFailClosed(t *testing.T) {
	calls := 0
	c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return reply(200, syntheticThread), nil
		}
		return reply(403, `{"error":{"code":403,"message":"SYNTHETIC_PRIVATE_RESPONSE"}}`), nil
	})})
	out, err := c.Thread(context.Background(), "thread1", 1)
	if err != nil || out.Complete || out.Messages[0].ReadError == "" || strings.Contains(out.Messages[0].ReadError, "PRIVATE") {
		t.Fatal(out, err)
	}
	c, _ = NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(*http.Request) (*http.Response, error) {
		return reply(200, strings.Repeat("x", maxContextResponse+1)), nil
	})})
	if _, err = c.Thread(context.Background(), "thread1", 1); err == nil {
		t.Fatal("oversized response accepted")
	}
}
func TestThreadIdentityMismatchAndInvalidInputsBeforeTransport(t *testing.T) {
	for _, data := range []string{strings.Replace(syntheticThread, `"id":"thread1"`, `"id":"wrong"`, 1), strings.Replace(syntheticThread, `"threadId":"thread1"`, `"threadId":"other"`, 1), strings.Replace(syntheticThread, `"id":"m1"`, `"id":"m3"`, 1)} {
		calls := 0
		c, _ := NewWithHTTP(context.Background(), &http.Client{Transport: roundtrip(func(*http.Request) (*http.Response, error) { calls++; return reply(200, data), nil })})
		if _, err := c.Thread(context.Background(), "thread1", 2); err == nil || calls != 1 {
			t.Fatal("bad identity accepted", err, calls)
		}
		before := calls
		if _, err := c.Thread(context.Background(), "../bad", 2); err == nil || calls != before {
			t.Fatal("invalid reached transport")
		}
	}
}
func TestPlainExcerptIgnoresAttachmentsHTMLAndReportsTruncation(t *testing.T) {
	plain := func(s string) *gmail.MessagePart {
		return &gmail.MessagePart{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString([]byte(s)), Size: int64(len(s))}}
	}
	attachment := plain("SYNTHETIC_ATTACHMENT")
	attachment.Filename = "secret.txt"
	part := &gmail.MessagePart{MimeType: "multipart/mixed", Parts: []*gmail.MessagePart{attachment, {MimeType: "text/html", Body: &gmail.MessagePartBody{Data: base64.RawURLEncoding.EncodeToString([]byte("<script>bad</script>"))}}, plain("context")}}
	if text, complete := plainExcerpt(part, 0); text != "context" || !complete {
		t.Fatal(text, complete)
	}
	part.Parts = append(part.Parts, plain("second inline text"))
	if text, complete := plainExcerpt(part, 0); text != "context" || complete {
		t.Fatal("omitted inline text claimed complete", text, complete)
	}
	if text, complete := plainExcerpt(plain(strings.Repeat("界", 1000)), 0); !utf8.ValidString(text) || len(text) > 2000 || complete {
		t.Fatal("unsafe truncation")
	}
}
