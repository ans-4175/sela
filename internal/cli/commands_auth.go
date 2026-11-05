package cli

import (
	"context"
	"errors"
	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/auth"
	"github.com/ans-4175/sela/internal/credentials"
	"github.com/ans-4175/sela/internal/googleapi"
	"github.com/ans-4175/sela/internal/localstore"
	"golang.org/x/oauth2"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func (a *App) accountCommand(ctx context.Context, args []string) error {
	p, e := a.registry()
	if e != nil {
		return e
	}
	if len(args) == 1 && args[0] == "list" {
		r, e := accounts.Load(p)
		if e != nil {
			return e
		}
		return encode(a.Out, r)
	}
	if args[0] != "add" {
		return errors.New("unsupported account command")
	}
	name, email := "", ""
	if e = parse("account add", args[1:], map[string]*string{"name": &name, "email": &email}, nil, nil); e != nil {
		return e
	}
	account := accounts.Account{Name: name, Email: email}
	if e = accounts.Add(ctx, p, account); e != nil {
		return e
	}
	return encode(a.Out, map[string]any{"account": account, "authenticated": false})
}
func (a *App) authCommand(ctx context.Context, cmd string, account accounts.Account, clientFile, profile string, reconsent, mailModify, freeBusy bool) error {
	dir, locks, e := a.paths()
	if e != nil {
		return e
	}
	if cmd == "login" {
		if profile != "gmail-calendar-read" && profile != "gmail-calendar-write" {
			return errors.New("explicit --profile gmail-calendar-read|gmail-calendar-write required; compose includes send")
		}
		data, e := os.ReadFile(clientFile)
		if e != nil {
			return errors.New("cannot read explicit Desktop client file")
		}
		cfg, e := auth.DesktopConfig(data)
		if e != nil {
			return e
		}
		write := profile == "gmail-calendar-write"
		cfg.Scopes = auth.ConnectionScopes(write, mailModify, freeBusy)
		ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: 30 * time.Second})
		token, e := auth.Login(ctx, cfg, a.Out, reconsent)
		if e != nil {
			return e
		}
		client, e := googleapi.New(ctx, oauth2.StaticTokenSource(token))
		if e != nil {
			return e
		}
		if e = client.Verify(ctx, account); e != nil {
			return e
		}
		if e = localstore.Dir(locks); e != nil {
			return e
		}
		e = localstore.WithLock(ctx, auth.LockPath(locks, account.CredentialKey()), func() error {
			if e := localstore.Write(filepath.Join(dir, account.Name+".json"), Connection{Ready: false, Client: data, CalendarWrite: write, MailModify: mailModify, CalendarFreeBusy: freeBusy}); e != nil {
				return e
			}
			if a.Credentials.Save(account.CredentialKey(), token) != nil {
				return credentials.ErrUnavailable
			}
			return localstore.Write(filepath.Join(dir, account.Name+".json"), Connection{Ready: true, Client: data, CalendarWrite: write, MailModify: mailModify, CalendarFreeBusy: freeBusy})
		})
		if e != nil {
			return e
		}
		return encode(a.Out, map[string]any{"account": account, "authenticated": true, "calendar_write": write, "mail_modify": mailModify, "calendar_freebusy": freeBusy})
	}
	if cmd == "status" {
		var exists bool
		e = localstore.WithLock(ctx, auth.LockPath(locks, account.CredentialKey()), func() error {
			_, err := a.Credentials.Load(account.CredentialKey())
			if errors.Is(err, credentials.ErrNotFound) {
				return nil
			}
			exists = err == nil
			return err
		})
		if e != nil {
			return e
		}
		return encode(a.Out, map[string]any{"account": account, "credential_present": exists, "live_identity_checked": false})
	}
	deletable, ok := a.Credentials.(interface{ Delete(string) error })
	if !ok {
		return credentials.ErrUnavailable
	}
	e = localstore.WithLock(ctx, auth.LockPath(locks, account.CredentialKey()), func() error { return deletable.Delete(account.CredentialKey()) })
	if e != nil {
		return e
	}
	return encode(a.Out, map[string]any{"account": account, "local_credential_deleted": true, "remote_token_revoked": false})
}
