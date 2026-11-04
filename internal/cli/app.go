package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/auth"
	"github.com/ans-4175/sela/internal/credentials"
	"github.com/ans-4175/sela/internal/googleapi"
	"github.com/ans-4175/sela/internal/localstore"
	"github.com/ans-4175/sela/internal/plans"
	"golang.org/x/oauth2"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Connection struct {
	Ready         bool            `json:"ready"`
	Client        json.RawMessage `json:"desktop_client"`
	CalendarWrite bool            `json:"calendar_write"`
	MailModify    bool            `json:"mail_modify,omitempty"`
}
type App struct {
	Out          io.Writer
	RegistryPath string
	Credentials  credentials.Store
	Confirm      func(plans.Plan) error
}

func Run(ctx context.Context, args []string, out io.Writer) error {
	return (&App{Out: out, Credentials: credentials.Keychain{}, Confirm: ConfirmTerminal}).Run(ctx, args)
}
func encode(out io.Writer, value any) error {
	e := json.NewEncoder(out)
	e.SetIndent("", "  ")
	return e.Encode(value)
}
func (a *App) registry() (string, error) {
	if a.RegistryPath != "" {
		return a.RegistryPath, nil
	}
	return accounts.DefaultPath()
}
func parse(name string, args []string, values map[string]*string, numbers map[string]*int64, booleans map[string]*bool) error {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	f.SetOutput(io.Discard)
	for n, p := range values {
		f.StringVar(p, n, *p, "")
	}
	for n, p := range numbers {
		f.Int64Var(p, n, *p, "")
	}
	for n, p := range booleans {
		f.BoolVar(p, n, *p, "")
	}
	if f.Parse(args) != nil || f.NArg() != 0 {
		return errors.New("invalid/unsupported arguments; see --help or --skill")
	}
	return nil
}
func (a *App) account(name string) (accounts.Account, error) {
	p, e := a.registry()
	if e != nil {
		return accounts.Account{}, e
	}
	r, e := accounts.Load(p)
	if e != nil {
		return accounts.Account{}, e
	}
	return r.Resolve(name)
}
func (a *App) paths() (string, string, error) {
	p, e := a.registry()
	if e != nil {
		return "", "", e
	}
	return filepath.Join(filepath.Dir(p), "connections"), filepath.Join(filepath.Dir(p), "locks"), nil
}
func (a *App) planStore() (plans.Store, error) {
	p, e := a.registry()
	return plans.Store{Dir: filepath.Join(filepath.Dir(p), "plans")}, e
}
func (a *App) connection(account accounts.Account) (Connection, *oauth2.Config, error) {
	var c Connection
	dir, _, e := a.paths()
	if e != nil {
		return c, nil, e
	}
	if e = localstore.Read(filepath.Join(dir, account.Name+".json"), &c); e != nil {
		return c, nil, errors.New("auth setup required for explicit account")
	}
	if !c.Ready {
		return c, nil, errors.New("authentication setup is incomplete; explicit login required")
	}
	cfg, e := auth.DesktopConfig(c.Client)
	if e != nil {
		return c, nil, e
	}
	cfg.Scopes = auth.ProfileScopes(c.CalendarWrite, c.MailModify)
	return c, cfg, nil
}
func (a *App) client(ctx context.Context, account accounts.Account) (*googleapi.Clients, Connection, error) {
	conn, cfg, e := a.connection(account)
	if e != nil {
		return nil, conn, e
	}
	_, locks, e := a.paths()
	if e != nil {
		return nil, conn, e
	}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: 30 * time.Second})
	source, e := auth.TokenSource(ctx, cfg, a.Credentials, account.CredentialKey(), locks, func() error {
		current, _, err := a.connection(account)
		if err != nil {
			return err
		}
		if string(current.Client) != string(conn.Client) || current.CalendarWrite != conn.CalendarWrite || current.MailModify != conn.MailModify {
			return errors.New("OAuth configuration changed; restart command")
		}
		return nil
	})
	if e != nil {
		return nil, conn, e
	}
	client, e := googleapi.New(ctx, source)
	if e != nil {
		return nil, conn, e
	}
	if e = client.Verify(ctx, account); e != nil {
		return nil, conn, e
	}
	return client, conn, nil
}
func readInput(path string, out any) error {
	f, e := os.Open(path)
	if e != nil {
		return errors.New("cannot open explicit input JSON")
	}
	defer f.Close()
	d := json.NewDecoder(io.LimitReader(f, 12<<20))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return errors.New("invalid input JSON or unsupported fields")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("invalid trailing input JSON")
	}
	return nil
}
