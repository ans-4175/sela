package actions

import (
	"encoding/base64"
	"github.com/ans-4175/sela/internal/accounts"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMailExactSnapshotAndHeaders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.txt")
	os.WriteFile(path, []byte("original attachment"), 0600)
	a := accounts.Account{Name: "personal", Email: "person@example.test"}
	input := MailInput{To: []string{"recipient@example.test"}, Bcc: []string{"copy@example.test"}, Subject: "Halo dunia", Body: "Exact body\nnext line", Attachments: []AttachmentInput{{path}}}
	m, e := BuildMail(a, input)
	if e != nil {
		t.Fatal(e)
	}
	os.WriteFile(path, []byte("changed"), 0600)
	if m.Validate(a) != nil || m.Validate(accounts.Account{Name: "other", Email: "other@example.test"}) == nil {
		t.Fatal("account/snapshot validation")
	}
	raw, _ := base64.RawURLEncoding.DecodeString(m.Raw)
	message, e := mail.ReadMessage(strings.NewReader(string(raw)))
	if e != nil {
		t.Fatal(e)
	}
	if message.Header.Get("From") != a.Email || message.Header.Get("Bcc") != "copy@example.test" {
		t.Fatal("sender/Bcc mismatch")
	}
	_, params, _ := mime.ParseMediaType(message.Header.Get("Content-Type"))
	reader := multipart.NewReader(message.Body, params["boundary"])
	part, _ := reader.NextPart()
	text, _ := io.ReadAll(part)
	decoded, _ := base64.StdEncoding.DecodeString(strings.ReplaceAll(strings.TrimSpace(string(text)), "\r\n", ""))
	if string(decoded) != input.Body {
		t.Fatal("body changed")
	}
	part, _ = reader.NextPart()
	data, _ := io.ReadAll(part)
	bytes, _ := base64.StdEncoding.DecodeString(strings.ReplaceAll(strings.TrimSpace(string(data)), "\r\n", ""))
	if string(bytes) != "original attachment" || m.Attachments[0].SHA256 != hash(bytes) {
		t.Fatal("attachment was not snapshotted")
	}
	input.Subject = "bad\r\nBcc:evil@example.test"
	if _, e = BuildMail(a, input); e == nil {
		t.Fatal("header injection")
	}
}
func TestEventScopeValidation(t *testing.T) {
	cp := Calendar{CalendarID: "primary", SendUpdates: "all", Event: &Event{Summary: "Fixture", Start: EventTime{"2026-12-01T10:00:00+07:00", "Asia/Jakarta"}, End: EventTime{"2026-12-01T11:00:00+07:00", "Asia/Jakarta"}, Attendees: []string{"person@example.test"}}}
	if e := cp.Validate("calendar.create"); e != nil {
		t.Fatal(e)
	}
	if cp.Validate("calendar.update") == nil {
		t.Fatal("update without ETag")
	}
	cp.Event.Start.TimeZone = "Asia/Tokyo"
	if cp.Validate("calendar.create") == nil {
		t.Fatal("conflicting timestamp offset and timezone")
	}
	cp.Event.Start.TimeZone = "Asia/Jakarta"
	for _, tc := range []struct {
		start, end string
		valid      bool
	}{
		{"2026-07-01T10:00:00-04:00", "2026-07-01T11:00:00-04:00", true},
		{"2026-12-01T10:00:00-05:00", "2026-12-01T11:00:00-05:00", true},
		{"2026-07-01T10:00:00-05:00", "2026-07-01T11:00:00-05:00", false},
	} {
		event := *cp.Event
		event.Start = EventTime{tc.start, "America/New_York"}
		event.End = EventTime{tc.end, "America/New_York"}
		candidate := cp
		candidate.Event = &event
		if (candidate.Validate("calendar.create") == nil) != tc.valid {
			t.Fatal("DST offset validation", tc.start)
		}
	}
	cp.SendUpdates = ""
	if cp.Validate("calendar.create") == nil {
		t.Fatal("implicit notification policy")
	}
	cp.SendUpdates = "none"
	cp.Event.End.DateTime = "2026-12-01T09:00:00+07:00"
	if cp.Validate("calendar.create") == nil {
		t.Fatal("negative duration")
	}
}
