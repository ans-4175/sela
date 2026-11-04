//go:build windows

package localstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsProtectedStateFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "must-not-exist", "state.json")
	called := false
	for _, err := range []error{Dir(filepath.Dir(path)), Read(path, &struct{}{}), Write(path, struct{}{}), WithLock(context.Background(), path, func() error { called = true; return nil })} {
		if !errors.Is(err, ErrUnsafe) {
			t.Fatal("protected state did not refuse", err)
		}
	}
	if called {
		t.Fatal("unsafe lock callback ran")
	}
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Fatal("refusal created state", err)
	}
}
