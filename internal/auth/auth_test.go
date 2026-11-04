//go:build darwin || linux

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/ans-4175/sela/internal/credentials"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func response(body string, status int) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{"Content-Type": {"application/json"}}}
}

type memory struct {
	mu      sync.Mutex
	t       *oauth2.Token
	saveErr bool
	saves   int
}

func (m *memory) Load(string) (*oauth2.Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.t == nil {
		return nil, credentials.ErrNotFound
	}
	v := *m.t
	return &v, nil
}
func (m *memory) Save(_ string, v *oauth2.Token) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveErr {
		return errors.New("SYNTHETIC_SECRET")
	}
	copy := *v
	m.t = &copy
	m.saves++
	return nil
}
func TestDesktopOnlyOfficialEndpointsAndPKCE(t *testing.T) {
	desktop := `{"installed":{"client_id":"synthetic.apps.googleusercontent.com","client_secret":"SYNTHETIC_SECRET","auth_uri":"` + google.Endpoint.AuthURL + `","token_uri":"` + google.Endpoint.TokenURL + `","redirect_uris":["http://localhost"]}}`
	c, e := DesktopConfig([]byte(desktop))
	if e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.Replace(desktop, "installed", "web", 1), strings.Replace(desktop, google.Endpoint.TokenURL, "https://attacker.test/token", 1), `{}`, `{"installed":null}`} {
		if _, e := DesktopConfig([]byte(bad)); e == nil {
			t.Fatal("bad OAuth config accepted")
		}
	}
	c.RedirectURL = "http://127.0.0.1:12345/callback"
	verifier := oauth2.GenerateVerifier()
	u, e := AuthorizationURL(c, strings.Repeat("s", 43), verifier, false)
	if e != nil {
		t.Fatal(e)
	}
	parsed, _ := url.Parse(u)
	q := parsed.Query()
	if q.Get("code_challenge_method") != "S256" || q.Get("access_type") != "offline" || q.Get("code_challenge") == verifier || q.Get("prompt") != "" {
		t.Fatal("PKCE/offline missing or consent forced by default")
	}
	consentURL, err := AuthorizationURL(c, strings.Repeat("s", 43), verifier, true)
	if err != nil {
		t.Fatal(err)
	}
	consent, _ := url.Parse(consentURL)
	if consent.Query().Get("prompt") != "consent" || consent.Query().Get("code_challenge") != q.Get("code_challenge") {
		t.Fatal("explicit re-consent lost prompt/PKCE")
	}
	c.RedirectURL = "https://attacker.test/callback"
	if _, e := AuthorizationURL(c, strings.Repeat("s", 43), verifier, false); e == nil {
		t.Fatal("nonloopback redirect")
	}
	if _, e := AuthorizationURL(c, "short", verifier, false); e == nil {
		t.Fatal("short state")
	}
}
func TestRefreshReloadSerializedAndPersistence(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	m := &memory{t: &oauth2.Token{AccessToken: "SYNTHETIC_OLD", RefreshToken: "SYNTHETIC_REFRESH", Expiry: time.Now().Add(-time.Hour)}}
	var requests atomic.Int32
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		requests.Add(1)
		r.ParseForm()
		if r.Form.Get("refresh_token") != "SYNTHETIC_REFRESH" {
			t.Error("incorrect account token")
		}
		return response(`{"access_token":"SYNTHETIC_NEW","token_type":"Bearer","expires_in":3600}`, 200), nil
	})})
	c := &oauth2.Config{ClientID: "synthetic", ClientSecret: "SYNTHETIC_SECRET", Endpoint: oauth2.Endpoint{TokenURL: "https://fixture.invalid/token", AuthStyle: oauth2.AuthStyleInParams}}
	one, _ := TokenSource(ctx, c, m, "personal:person@example.test", dir, nil)
	two, _ := TokenSource(ctx, c, m, "personal:person@example.test", dir, nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s := one
			if i%2 == 1 {
				s = two
			}
			token, e := s.Token()
			if e != nil || token.RefreshToken != "SYNTHETIC_REFRESH" || token.AccessToken != "SYNTHETIC_NEW" {
				t.Error("refresh result", e)
			}
		}(i)
	}
	wg.Wait()
	if requests.Load() != 1 || m.saves != 1 {
		t.Fatal("refresh not shared", requests.Load(), m.saves)
	}
	m.mu.Lock()
	m.t.Expiry = time.Now().Add(-time.Hour)
	m.saveErr = true
	m.mu.Unlock()
	if token, e := one.Token(); e == nil || token != nil || strings.Contains(e.Error(), "SYNTHETIC") {
		t.Fatal("failed persistence exposed token/error")
	}
	before := requests.Load()
	one.Token()
	if requests.Load() != before {
		t.Fatal("failed source not latched")
	}
}
func TestTokenGuardAndInvalidGrantSafe(t *testing.T) {
	dir, _ := filepath.EvalSymlinks(t.TempDir())
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	m := &memory{t: &oauth2.Token{AccessToken: "old", RefreshToken: "SYNTHETIC_REFRESH", Expiry: time.Now().Add(-time.Hour)}}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		return response(`{"error":"invalid_grant","error_description":"SYNTHETIC_PRIVATE_RESPONSE"}`, 400), nil
	})})
	c := &oauth2.Config{ClientID: "synthetic", Endpoint: oauth2.Endpoint{TokenURL: "https://fixture.invalid", AuthStyle: oauth2.AuthStyleInParams}}
	s, _ := TokenSource(ctx, c, m, "account", dir, func() error { return errors.New("configuration pending") })
	if _, e := s.Token(); e == nil || e.Error() != "configuration pending" {
		t.Fatal(e)
	}
	s, _ = TokenSource(ctx, c, m, "account", dir, nil)
	if _, e := s.Token(); e == nil || !strings.Contains(e.Error(), "invalid_grant") || strings.Contains(e.Error(), "PRIVATE") {
		t.Fatal(e)
	}
}
func TestCallbackStateHostAndReplay(t *testing.T) {
	c := NewCallback(strings.Repeat("s", 43), "127.0.0.1:1234")
	request := func(path, host string) int {
		r := httptest.NewRequest("GET", path, nil)
		r.Host = host
		w := httptest.NewRecorder()
		c.ServeHTTP(w, r)
		return w.Code
	}
	if request("/callback?state=bad&code=synthetic", "127.0.0.1:1234") != 400 {
		t.Fatal("bad state")
	}
	if request("/callback?state="+c.state+"&code=synthetic", "attacker.test") != 400 {
		t.Fatal("bad host")
	}
	if request("/callback?state="+c.state+"&code=synthetic", "127.0.0.1:1234") != 200 {
		t.Fatal("valid callback")
	}
	if <-c.codes != "synthetic" {
		t.Fatal("code")
	}
	if request("/callback?state="+c.state+"&code=again", "127.0.0.1:1234") != 409 {
		t.Fatal("replay")
	}
}
func TestGrantedScopesFailClosed(t *testing.T) {
	token := (&oauth2.Token{}).WithExtra(map[string]any{"scope": strings.Join(ProfileScopes(false, false), " ")})
	if e := ValidateScopes(token, ProfileScopes(false, false)); e != nil {
		t.Fatal(e)
	}
	if ValidateScopes(token, ProfileScopes(true, false)) == nil {
		t.Fatal("silent calendar scope widening")
	}
	if ValidateScopes(&oauth2.Token{}, Scopes()) == nil {
		t.Fatal("missing scope evidence")
	}
	if ValidateScopes(token, ProfileScopes(false, true)) == nil {
		t.Fatal("mail modify silently enabled without consent")
	}
	modify := (&oauth2.Token{}).WithExtra(map[string]any{"scope": strings.Join(ProfileScopes(false, true), " ")})
	if err := ValidateScopes(modify, ProfileScopes(false, true)); err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(ProfileScopes(false, false))
	if strings.Contains(string(b), MailModifyScope) {
		t.Fatal("mail modify leaked into default scopes")
	}
	if strings.Contains(string(b), "calendar.events\"") {
		t.Fatal("default write scope")
	}
}
