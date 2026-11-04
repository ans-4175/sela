package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"golang.org/x/oauth2"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Callback struct {
	state, host string
	once        sync.Once
	codes       chan string
	denied      chan struct{}
}

func NewCallback(state, host string) *Callback {
	return &Callback{state: state, host: host, codes: make(chan string, 1), denied: make(chan struct{}, 1)}
}
func (c *Callback) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	q := r.URL.Query()
	if r.Method != "GET" || r.URL.Path != "/callback" || r.Host != c.host || len(q["state"]) != 1 || subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(c.state)) != 1 {
		http.Error(w, "Invalid OAuth callback", 400)
		return
	}
	code := q.Get("code")
	if len(q["code"]) > 1 || strings.ContainsAny(code, "\r\n") || len(code) > 8192 {
		http.Error(w, "Invalid OAuth code", 400)
		return
	}
	accepted := false
	c.once.Do(func() {
		accepted = true
		if q.Get("error") != "" || code == "" {
			c.denied <- struct{}{}
		} else {
			c.codes <- code
		}
	})
	if !accepted {
		http.Error(w, "Callback already consumed", 409)
		return
	}
	io.WriteString(w, "Consent received. Return to sela; account identity will be verified before saving.")
}

// Login starts loopback only on an explicit CLI login. It never stores a token itself.
func Login(ctx context.Context, config *oauth2.Config, out io.Writer, reconsent bool) (*oauth2.Token, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("cannot open loopback OAuth callback")
	}
	defer listener.Close()
	random := make([]byte, 32)
	if _, err = rand.Read(random); err != nil {
		return nil, errors.New("cannot generate OAuth state")
	}
	state := base64.RawURLEncoding.EncodeToString(random)
	verifier := oauth2.GenerateVerifier()
	clone := *config
	clone.RedirectURL = "http://" + listener.Addr().String() + "/callback"
	handler := NewCallback(state, listener.Addr().String())
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, MaxHeaderBytes: 16384}
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		server.Shutdown(shutdown)
	}()
	authURL, err := AuthorizationURL(&clone, state, verifier, reconsent)
	if err != nil {
		return nil, err
	}
	if _, err = fmt.Fprintf(out, "Open this URL in your browser to consent to the explicitly selected scopes:\n%s\n", authURL); err != nil {
		return nil, errors.New("cannot display OAuth consent URL")
	}
	serving := make(chan error, 1)
	go func() { serving <- server.Serve(listener) }()
	var code string
	select {
	case code = <-handler.codes:
	case <-handler.denied:
		return nil, errors.New("OAuth consent denied or incomplete")
	case <-ctx.Done():
		return nil, errors.New("OAuth login cancelled or timed out")
	case <-serving:
		return nil, errors.New("OAuth callback stopped")
	}
	token, err := clone.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, safeOAuthError(err)
	}
	if token.RefreshToken == "" {
		return nil, errors.New("OAuth returned no offline refresh credential; repeat the explicit auth login command with --reauth to request fresh consent")
	}
	if err = ValidateScopes(token, clone.Scopes); err != nil {
		return nil, err
	}
	return token, nil
}
