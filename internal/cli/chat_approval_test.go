//go:build darwin || linux

package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatApprovalInvalidFlagsAreNoninteractiveAndLocal(t *testing.T) {
	app, _, creds := fixture(t)
	for _, args := range [][]string{
		{"plan", "approve", "--account", "unknown", "--id", strings.Repeat("a", 32)},
		{"plan", "approve", "--account", "unknown", "--id", strings.Repeat("a", 32), "--hash", strings.Repeat("b", 64)},
		{"plan", "approve", "--account", "unknown", "--id", strings.Repeat("a", 32), "--hash", strings.Repeat("b", 64), "--user-approval", "  "},
		{"plan", "approve", "--account", "unknown", "--id", strings.Repeat("a", 32), "--yes"},
	} {
		if err := app.Run(context.Background(), args); err == nil || strings.Contains(err.Error(), "account alias") {
			t.Fatal("invalid consent reached account resolution or prompted", args, err)
		}
	}
	if creds.calls != 0 {
		t.Fatal("approval touched credentials")
	}
	if _, err := os.Stat(filepath.Dir(app.RegistryPath)); !os.IsNotExist(err) {
		t.Fatal("invalid consent created state")
	}
}
