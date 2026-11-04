// Package accounts stores nonsecret metadata; credentials belong exclusively in Keychain.
package accounts

import (
	"context"
	"errors"
	"github.com/ans-4175/sela/internal/localstore"
	"net/mail"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Account struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

var alias = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func (a Account) Validate() error {
	if !alias.MatchString(a.Name) {
		return errors.New("invalid account alias (1-32 lowercase letters/digits/_/-, starting with a letter)")
	}
	addr, err := mail.ParseAddress(a.Email)
	if err != nil || addr.Address != a.Email || strings.TrimSpace(a.Email) != a.Email || strings.ContainsAny(a.Email, "\r\n\x00") {
		return errors.New("account email must be a plain email address")
	}
	return nil
}
func (a Account) CredentialKey() string { return a.Name + ":" + strings.ToLower(a.Email) }

type Registry struct {
	Version  int       `json:"version"`
	Accounts []Account `json:"accounts"`
}

func DefaultPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", errors.New("home unavailable")
		}
		base = filepath.Join(home, ".config")
	}
	if !filepath.IsAbs(base) {
		return "", errors.New("XDG_CONFIG_HOME must be absolute")
	}
	return filepath.Join(base, "sela", "accounts.json"), nil
}
func Load(path string) (Registry, error) {
	r := Registry{Version: 1, Accounts: []Account{}}
	err := localstore.Read(path, &r)
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	if r.Version != 1 {
		return r, errors.New("unsupported registry version")
	}
	seen := Registry{Version: 1, Accounts: []Account{}}
	for _, a := range r.Accounts {
		if err := seen.Add(a); err != nil {
			return r, err
		}
	}
	return seen, nil
}
func (r Registry) Resolve(name string) (Account, error) {
	if name == "" {
		return Account{}, errors.New("--account is required; no implicit default")
	}
	for _, a := range r.Accounts {
		if a.Name == name {
			return a, nil
		}
	}
	return Account{}, errors.New("unknown account alias")
}
func (r *Registry) Add(a Account) error {
	if err := a.Validate(); err != nil {
		return err
	}
	for _, v := range r.Accounts {
		if v.Name == a.Name {
			return errors.New("alias exists; refusing to rebind")
		}
		if strings.EqualFold(v.Email, a.Email) {
			return errors.New("email already registered")
		}
	}
	r.Accounts = append(r.Accounts, a)
	return nil
}
func Add(ctx context.Context, path string, a Account) error {
	return localstore.WithLock(ctx, path+".lock", func() error {
		r, err := Load(path)
		if err != nil {
			return err
		}
		if err = r.Add(a); err != nil {
			return err
		}
		return localstore.Write(path, r)
	})
}
