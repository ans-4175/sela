//go:build windows

// Windows private storage deliberately fails closed until ACL and locking tests exist.
// Stateless help/skill/version work; no pretend 0600 guarantee on Windows NTFS.
package localstore

import (
	"context"
	"errors"
)

var ErrUnsafe = errors.New("Windows private storage is not enabled: ACL/locking runtime validation required; no plaintext credential fallback")

func Dir(string) error                                     { return ErrUnsafe }
func Read(string, any) error                               { return ErrUnsafe }
func Write(string, any) error                              { return ErrUnsafe }
func WithLock(context.Context, string, func() error) error { return ErrUnsafe }
