//go:build darwin || linux

package accounts

import (
	"context"
	"fmt"
	"github.com/ans-4175/sela/internal/localstore"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestRegistryIsolationAndConcurrentAdds(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "accounts.json")
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if e := Add(context.Background(), path, Account{Name: fmt.Sprintf("a%d", i), Email: fmt.Sprintf("user%d@example.test", i)}); e != nil {
				t.Error(e)
			}
		}(i)
	}
	wg.Wait()
	r, e := Load(path)
	if e != nil || len(r.Accounts) != 24 {
		t.Fatal(e, len(r.Accounts))
	}
	if _, e = r.Resolve(""); e == nil {
		t.Fatal("implicit default")
	}
	if Add(context.Background(), path, Account{Name: "a1", Email: "replacement@example.test"}) == nil {
		t.Fatal("alias rebound")
	}
	if Add(context.Background(), path, Account{Name: "other", Email: "USER1@example.test"}) == nil {
		t.Fatal("duplicate email")
	}
	a, _ := r.Resolve("a1")
	b, _ := r.Resolve("a2")
	if a.CredentialKey() == b.CredentialKey() {
		t.Fatal("account credential collision")
	}
}
func TestRegistryValidationAndPaths(t *testing.T) {
	for _, a := range []Account{{Name: "../x", Email: "x@example.test"}, {Name: "x", Email: "Name <x@example.test>"}, {Name: "x", Email: "x@example.test\r\nsecret"}} {
		if a.Validate() == nil {
			t.Fatal("accepted invalid account")
		}
	}
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "accounts.json")
	if e := localstore.Write(path, Registry{Version: 1, Accounts: []Account{{"a", "same@example.test"}, {"b", "SAME@example.test"}}}); e != nil {
		t.Fatal(e)
	}
	if _, e := Load(path); e == nil {
		t.Fatal("duplicate persisted accounts accepted")
	}
	t.Setenv("XDG_CONFIG_HOME", "relative")
	if _, e := DefaultPath(); e == nil {
		t.Fatal("relative path accepted")
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	if p, e := DefaultPath(); e != nil || p != filepath.Join(dir, "sela", "accounts.json") {
		t.Fatal(p, e)
	}
}
