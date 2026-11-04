//go:build darwin || linux

package localstore

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func private(t *testing.T) string {
	t.Helper()
	p, e := filepath.EvalSymlinks(t.TempDir())
	if e != nil {
		t.Fatal(e)
	}
	if err := os.Chmod(p, 0700); err != nil {
		t.Fatal(err)
	}
	return p
}
func TestSafeFilesAndSymlinks(t *testing.T) {
	root := private(t)
	path := filepath.Join(root, "a.json")
	if e := Write(path, map[string]int{"n": 1}); e != nil {
		t.Fatal(e)
	}
	var out map[string]int
	if e := Read(path, &out); e != nil || out["n"] != 1 {
		t.Fatal(e, out)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatal(info.Mode())
	}
	if e := os.Symlink(path, filepath.Join(root, "link.json")); e != nil {
		t.Fatal(e)
	}
	if Read(filepath.Join(root, "link.json"), &out) == nil || Write(filepath.Join(root, "link.json"), out) == nil {
		t.Fatal("symlink accepted")
	}
	if e := os.Link(path, filepath.Join(root, "hard.json")); e != nil {
		t.Fatal(e)
	}
	if Read(path, &out) == nil {
		t.Fatal("hardlink accepted")
	}
}
func TestRejectUnsafeDirectoryAndTrailingJSON(t *testing.T) {
	root := private(t)
	unsafe := filepath.Join(root, "unsafe")
	os.Mkdir(unsafe, 0755)
	if Dir(unsafe) == nil {
		t.Fatal("public final dir accepted")
	}
	target := filepath.Join(root, "target")
	os.Mkdir(target, 0700)
	link := filepath.Join(root, "link")
	os.Symlink(target, link)
	if Dir(filepath.Join(link, "nested")) == nil {
		t.Fatal("ancestor symlink accepted")
	}
	path := filepath.Join(root, "bad.json")
	os.WriteFile(path, []byte(`{} {}`), 0600)
	var v any
	if Read(path, &v) == nil {
		t.Fatal("trailing JSON accepted")
	}
}
func TestLockCancellationAndExclusion(t *testing.T) {
	root := private(t)
	path := filepath.Join(root, "x.lock")
	entered := make(chan struct{})
	release := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if e := WithLock(context.Background(), path, func() error { close(entered); <-release; return nil }); e != nil {
			t.Error(e)
		}
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		close(release)
		wg.Wait()
		t.Fatal("lock not entered")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if WithLock(ctx, path, func() error { t.Error("overlapping critical section"); return nil }) == nil {
		t.Fatal("cancelled lock succeeded")
	}
	close(release)
	wg.Wait()
}
func TestCrossProcessLock(t *testing.T) {
	if root := os.Getenv("GCLI_LOCK_TEST_ROOT"); root != "" {
		for i := 0; i < 15; i++ {
			e := WithLock(context.Background(), filepath.Join(root, "x.lock"), func() error {
				v := struct {
					N int `json:"n"`
				}{}
				if e := Read(filepath.Join(root, "v.json"), &v); e != nil {
					return e
				}
				v.N++
				return Write(filepath.Join(root, "v.json"), v)
			})
			if e != nil {
				t.Fatal(e)
			}
		}
		return
	}
	root := private(t)
	if e := Write(filepath.Join(root, "v.json"), map[string]int{"n": 0}); e != nil {
		t.Fatal(e)
	}
	commands := []*exec.Cmd{}
	for i := 0; i < 3; i++ {
		cmd := exec.Command(os.Args[0], "-test.run=^TestCrossProcessLock$")
		cmd.Env = append(os.Environ(), "GCLI_LOCK_TEST_ROOT="+root)
		if e := cmd.Start(); e != nil {
			t.Fatal(e)
		}
		commands = append(commands, cmd)
	}
	for _, cmd := range commands {
		if e := cmd.Wait(); e != nil {
			t.Fatal(e)
		}
	}
	data, _ := os.ReadFile(filepath.Join(root, "v.json"))
	var got struct{ N int }
	json.Unmarshal(data, &got)
	if got.N != 45 {
		t.Fatalf("lost updates: %d", got.N)
	}
}
