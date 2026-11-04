//go:build darwin || linux

package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/actions"
	"github.com/ans-4175/sela/internal/plans"
)

func TestMeetPreviewIsLocalUniqueAndHashBound(t *testing.T) {
	app, out, creds := fixture(t)
	ctx := context.Background()
	if err := accounts.Add(ctx, app.RegistryPath, accounts.Account{Name: "personal", Email: "person@example.test"}); err != nil {
		t.Fatal(err)
	}
	cp := actions.Calendar{CalendarID: "primary", SendUpdates: "all", Conference: &actions.Conference{Type: "google_meet"}, Event: &actions.Event{Summary: "Fixture", Start: actions.EventTime{DateTime: "2026-12-01T10:00:00+07:00", TimeZone: "Asia/Jakarta"}, End: actions.EventTime{DateTime: "2026-12-01T11:00:00+07:00", TimeZone: "Asia/Jakarta"}, Attendees: []string{"guest@example.test"}}}
	file := filepath.Join(filepath.Dir(app.RegistryPath), "event.json")
	write := func() {
		t.Helper()
		b, _ := json.Marshal(cp)
		if err := os.WriteFile(file, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var lastRequest string
	for i := 0; i < 2; i++ {
		write()
		out.Reset()
		if err := app.Run(ctx, []string{"calendar", "preview", "--account", "personal", "--action", "create", "--file", file}); err != nil {
			t.Fatal(err)
		}
		var result struct {
			Plan    plans.Plan `json:"plan"`
			Effects string     `json:"effects"`
		}
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		var payload actions.Calendar
		json.Unmarshal(result.Plan.Scope.Payload, &payload)
		if payload.Conference == nil || len(payload.Conference.RequestID) != 32 || payload.Conference.RequestID == lastRequest || result.Plan.State != "prepared" {
			t.Fatal("missing or reused request ID")
		}
		if !strings.Contains(result.Effects, "invitations can exist before") || !strings.Contains(result.Effects, "not Meet readiness") {
			t.Fatal("partial success risk not previewed")
		}
		lastRequest = payload.Conference.RequestID
		scope := result.Plan.Scope
		payload.Conference.RequestID = strings.Repeat("f", 32)
		scope.Payload, _ = json.Marshal(payload)
		if plans.Digest(scope) == result.Plan.Hash {
			t.Fatal("conference request not bound to approval hash")
		}
	}
	// User-supplied IDs and any update conference intent fail before credentials/API.
	cp.Conference.RequestID = strings.Repeat("a", 32)
	write()
	if err := app.Run(ctx, []string{"calendar", "preview", "--account", "personal", "--action", "create", "--file", file}); err == nil || !strings.Contains(err.Error(), "do not supply") {
		t.Fatal("supplied ID accepted", err)
	}
	cp.Conference.RequestID = ""
	cp.EventID = "fixture"
	write()
	if err := app.Run(ctx, []string{"calendar", "preview", "--account", "personal", "--action", "update", "--file", file}); err == nil || !strings.Contains(err.Error(), "create only") {
		t.Fatal("conference-changing update accepted", err)
	}
	if creds.calls != 0 {
		t.Fatal("Meet preview touched credentials")
	}
}
