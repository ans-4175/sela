// Package auth implements explicit Desktop OAuth, never ADC/ambient credentials.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/ans-4175/sela/internal/credentials"
	"github.com/ans-4175/sela/internal/localstore"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func Scopes() []string { return ProfileScopes(false) }
func ProfileScopes(calendarWrite bool) []string {
	calendarScope := "https://www.googleapis.com/auth/calendar.events.readonly"
	if calendarWrite {
		calendarScope = "https://www.googleapis.com/auth/calendar.events"
	}
	return []string{"openid", "email", "https://www.googleapis.com/auth/gmail.readonly", "https://www.googleapis.com/auth/gmail.compose", calendarScope, "https://www.googleapis.com/auth/calendar.calendarlist.readonly"}
}
func DesktopConfig(data []byte) (*oauth2.Config, error) {
	var doc struct {
		Installed json.RawMessage `json:"installed"`
		Web       json.RawMessage `json:"web"`
	}
	if json.Unmarshal(data, &doc) != nil || len(doc.Installed) == 0 || string(doc.Installed) == "null" || len(doc.Web) > 0 {
		return nil, errors.New("require a Google Desktop installed-client JSON, not a Web/service client")
	}
	c, err := google.ConfigFromJSON(data, Scopes()...)
	if err != nil || c.ClientID == "" || c.ClientSecret == "" || c.Endpoint.AuthURL != google.Endpoint.AuthURL || c.Endpoint.TokenURL != google.Endpoint.TokenURL {
		return nil, errors.New("invalid Google Desktop OAuth client/endpoints")
	}
	c.RedirectURL = ""
	return c, nil
}

var verifierPattern = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)

func AuthorizationURL(c *oauth2.Config, state, verifier string, reconsent bool) (string, error) {
	if c == nil || c.ClientID == "" || len(state) < 32 || !verifierPattern.MatchString(verifier) {
		return "", errors.New("incomplete OAuth state/PKCE setup")
	}
	u, err := url.Parse(c.RedirectURL)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "/callback" {
		return "", errors.New("OAuth redirect must be the local loopback callback")
	}
	options := []oauth2.AuthCodeOption{oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier)}
	if reconsent {
		options = append(options, oauth2.SetAuthURLParam("prompt", "consent"))
	}
	return c.AuthCodeURL(state, options...), nil
}
func ValidateScopes(t *oauth2.Token, requested []string) error {
	s, ok := t.Extra("scope").(string)
	if !ok || s == "" {
		return errors.New("OAuth did not report granted scopes; no credential saved")
	}
	got := map[string]bool{}
	for _, v := range strings.Fields(s) {
		got[v] = true
	}
	for _, v := range requested {
		if !got[v] && !(v == "email" && got["https://www.googleapis.com/auth/userinfo.email"]) {
			return errors.New("required OAuth scope was not granted; no credential saved")
		}
	}
	return nil
}
func LockPath(dir, key string) string {
	h := sha256.Sum256([]byte(key))
	return filepath.Join(dir, "token-"+hex.EncodeToString(h[:])+".lock")
}
func TokenSource(ctx context.Context, c *oauth2.Config, store credentials.Store, key, lockDir string, guard func() error) (oauth2.TokenSource, error) {
	if c == nil || c.ClientID == "" || key == "" {
		return nil, errors.New("explicit OAuth client/account required")
	}
	if err := localstore.Dir(lockDir); err != nil {
		return nil, err
	}
	return &lockedSource{ctx: ctx, config: c, store: store, key: key, lockDir: lockDir, guard: guard}, nil
}

type lockedSource struct {
	mu           sync.Mutex
	ctx          context.Context
	config       *oauth2.Config
	store        credentials.Store
	key, lockDir string
	failed       bool
	guard        func() error
}

func (s *lockedSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failed {
		return nil, errors.New("credential persistence previously failed; stop and check authentication")
	}
	var result *oauth2.Token
	err := localstore.WithLock(s.ctx, LockPath(s.lockDir, s.key), func() error {
		if s.guard != nil {
			if err := s.guard(); err != nil {
				return err
			}
		}
		seed, err := s.store.Load(s.key)
		if err != nil {
			return err
		}
		if seed == nil || seed.RefreshToken == "" {
			return credentials.ErrInvalid
		}
		// Reload under the cross-process lock, not from an earlier process-local cache.
		next, err := s.config.TokenSource(s.ctx, seed).Token()
		if err != nil {
			return safeOAuthError(err)
		}
		if next == nil || next.AccessToken == "" {
			return credentials.ErrInvalid
		}
		copy := *next
		if copy.RefreshToken == "" {
			copy.RefreshToken = seed.RefreshToken
		}
		if copy.AccessToken != seed.AccessToken || copy.RefreshToken != seed.RefreshToken || copy.TokenType != seed.TokenType || !copy.Expiry.Equal(seed.Expiry) {
			if s.store.Save(s.key, &copy) != nil {
				s.failed = true
				return errors.New("cannot persist refreshed credential; stopping")
			}
		}
		result = &copy
		return nil
	})
	return result, err
}
func safeOAuthError(err error) error {
	var e *oauth2.RetrieveError
	if errors.As(err, &e) && e.ErrorCode == "invalid_grant" {
		return errors.New("OAuth invalid_grant; explicit reauthentication required")
	}
	return errors.New("OAuth request failed; no credential details disclosed")
}
