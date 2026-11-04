// Package credentials uses the native OS keyring only, never plaintext fallback.
package credentials

import (
	"encoding/json"
	"errors"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

var ErrNotFound = errors.New("no saved credential")
var ErrUnavailable = errors.New("OS credential store unavailable; no plaintext fallback")
var ErrInvalid = errors.New("invalid stored credential")

type Store interface {
	Load(string) (*oauth2.Token, error)
	Save(string, *oauth2.Token) error
}
type Keychain struct{}

const service = "sela.oauth2"

func (Keychain) Load(key string) (*oauth2.Token, error) {
	v, e := keyring.Get(service, key)
	if errors.Is(e, keyring.ErrNotFound) {
		return nil, ErrNotFound
	}
	if e != nil {
		return nil, ErrUnavailable
	}
	var t oauth2.Token
	if json.Unmarshal([]byte(v), &t) != nil || t.RefreshToken == "" {
		return nil, ErrInvalid
	}
	return &t, nil
}
func (Keychain) Save(key string, t *oauth2.Token) error {
	if t == nil || t.RefreshToken == "" {
		return ErrInvalid
	}
	data, e := json.Marshal(t)
	if e != nil {
		return ErrInvalid
	}
	if keyring.Set(service, key, string(data)) != nil {
		return ErrUnavailable
	}
	return nil
}
func (Keychain) Delete(key string) error {
	e := keyring.Delete(service, key)
	if errors.Is(e, keyring.ErrNotFound) {
		return ErrNotFound
	}
	if e != nil {
		return ErrUnavailable
	}
	return nil
}
