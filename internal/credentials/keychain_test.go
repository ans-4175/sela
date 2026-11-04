package credentials

import (
	"errors"
	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
	"testing"
)

// MockInit MUST happen before ANY adapter call: never touch host Keychain.
func TestNativeAdapterWithMemoryProviderOnly(t *testing.T) {
	keyring.MockInit()
	k := Keychain{}
	if _, e := k.Load("fixture"); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	if e := k.Save("fixture", &oauth2.Token{AccessToken: "SYNTHETIC_ACCESS", RefreshToken: "SYNTHETIC_REFRESH"}); e != nil {
		t.Fatal(e)
	}
	v, e := k.Load("fixture")
	if e != nil || v.RefreshToken != "SYNTHETIC_REFRESH" {
		t.Fatal("roundtrip failed")
	}
	if e = k.Save("invalid", &oauth2.Token{}); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	keyring.Set(service, "malformed", "{not json}")
	if _, e = k.Load("malformed"); !errors.Is(e, ErrInvalid) {
		t.Fatal(e)
	}
	if e = k.Delete("fixture"); e != nil {
		t.Fatal(e)
	}
	if _, e = k.Load("fixture"); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
}
