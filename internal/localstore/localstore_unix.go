//go:build darwin || linux

// Package localstore provides private local JSON files and interprocess Unix locks.
// It is not a sandbox against other processes running as the same user.
package localstore

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

var ErrUnsafe = errors.New("unsafe local storage path or permissions")

func Dir(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return ErrUnsafe
	}
	current := string(filepath.Separator)
	for _, part := range strings.Split(strings.TrimPrefix(path, current), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			if err = os.Mkdir(current, 0700); err != nil && !errors.Is(err, os.ErrExist) {
				return ErrUnsafe
			}
			info, err = os.Lstat(current)
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return ErrUnsafe
		}
		st, ok := info.Sys().(*syscall.Stat_t)
		if !ok || (st.Uid != uint32(os.Getuid()) && st.Uid != 0) {
			return ErrUnsafe
		}
		// Root-owned sticky temp ancestors are allowed; the final private directory isn't.
		if info.Mode().Perm()&0022 != 0 && !(info.Mode()&os.ModeSticky != 0 && st.Uid == 0 && current != path) {
			return ErrUnsafe
		}
		if current == path && (st.Uid != uint32(os.Getuid()) || info.Mode().Perm() != 0700) {
			return ErrUnsafe
		}
	}
	return nil
}

func open(path string, flags int) (*os.File, error) {
	if err := Dir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	fd, err := unix.Open(path, flags|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		if errors.Is(err, unix.ENOENT) {
			return nil, os.ErrNotExist
		}
		return nil, ErrUnsafe
	}
	f := os.NewFile(uintptr(fd), path)
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, ErrUnsafe
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 || st.Nlink != 1 || st.Uid != uint32(os.Getuid()) {
		f.Close()
		return nil, ErrUnsafe
	}
	return f, nil
}

func Read(path string, out any) error {
	f, err := open(path, unix.O_RDONLY)
	if err != nil {
		return err
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 32<<20))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return errors.New("invalid local JSON state")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("invalid trailing JSON state")
	}
	return nil
}

func Write(path string, value any) error {
	if err := Dir(filepath.Dir(path)); err != nil {
		return err
	}
	if f, err := open(path, unix.O_RDONLY); err == nil {
		f.Close()
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return errors.New("cannot encode local state")
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sela-*")
	if err != nil {
		return errors.New("cannot create local state")
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	if tmp.Chmod(0600) != nil {
		return ErrUnsafe
	}
	if _, err = tmp.Write(append(data, '\n')); err != nil {
		return errors.New("cannot write local state")
	}
	if tmp.Sync() != nil || tmp.Close() != nil {
		return errors.New("cannot persist local state")
	}
	if os.Rename(tmp.Name(), path) != nil {
		return errors.New("cannot commit local state")
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return errors.New("cannot sync local directory")
	}
	defer dir.Close()
	if dir.Sync() != nil {
		return errors.New("cannot sync local directory")
	}
	return nil
}

func WithLock(ctx context.Context, path string, fn func() error) error {
	f, err := open(path, unix.O_RDWR|unix.O_CREAT)
	if err != nil {
		return err
	}
	defer f.Close()
	for {
		if err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			break
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			return errors.New("local lock unavailable")
		}
		select {
		case <-ctx.Done():
			return errors.New("local lock cancelled")
		case <-time.After(10 * time.Millisecond):
		}
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
	if ctx.Err() != nil {
		return errors.New("operation cancelled")
	}
	return fn()
}
