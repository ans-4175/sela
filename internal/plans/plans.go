// Package plans provides expiring, exact-payload, one-attempt local approvals.
package plans

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/ans-4175/sela/internal/accounts"
	"github.com/ans-4175/sela/internal/localstore"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

type Scope struct {
	ID      string           `json:"id"`
	Account accounts.Account `json:"account"`
	Action  string           `json:"action"`
	Payload json.RawMessage  `json:"payload"`
	Created time.Time        `json:"created"`
	Expires time.Time        `json:"expires"`
}
type Plan struct {
	Scope       Scope           `json:"scope"`
	Hash        string          `json:"sha256"`
	State       string          `json:"state"`
	ApprovedAt  *time.Time      `json:"approved_at,omitempty"`
	AttemptedAt *time.Time      `json:"attempted_at,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}
type Store struct {
	Dir string
	Now func() time.Time
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}
func Digest(scope Scope) string {
	b, _ := json.Marshal(scope)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

var idPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func (s Store) path(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", errors.New("invalid plan ID")
	}
	return filepath.Join(s.Dir, id+".json"), nil
}
func (s Store) locked(ctx context.Context, fn func() error) error {
	return localstore.WithLock(ctx, filepath.Join(s.Dir, "plans.lock"), fn)
}
func (s Store) Create(ctx context.Context, a accounts.Account, action string, payload any) (Plan, error) {
	var p Plan
	if a.Validate() != nil {
		return p, errors.New("explicit valid plan account required")
	}
	switch action {
	case "mail.send", "draft.create", "calendar.create", "calendar.update", "calendar.delete":
	default:
		return p, errors.New("unsupported plan action")
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) > 24<<20 {
		return p, errors.New("invalid or oversized action payload")
	}
	bytes := make([]byte, 16)
	if _, err = rand.Read(bytes); err != nil {
		return p, errors.New("cannot generate plan ID")
	}
	now := s.now()
	p = Plan{Scope: Scope{ID: hex.EncodeToString(bytes), Account: a, Action: action, Payload: data, Created: now, Expires: now.Add(15 * time.Minute)}, State: "prepared"}
	p.Hash = Digest(p.Scope)
	path, _ := s.path(p.Scope.ID)
	err = s.locked(ctx, func() error {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.New("plan collision; nothing written")
		}
		return localstore.Write(path, p)
	})
	return p, err
}
func (s Store) load(id string) (Plan, error) {
	var p Plan
	path, err := s.path(id)
	if err != nil {
		return p, err
	}
	if err = localstore.Read(path, &p); err != nil {
		return p, err
	}
	if p.Scope.ID != id || p.Scope.Account.Validate() != nil || p.Hash != Digest(p.Scope) || p.Scope.Expires.Sub(p.Scope.Created) != 15*time.Minute {
		return p, errors.New("plan scope/hash integrity check failed")
	}
	return p, nil
}
func (s Store) Load(ctx context.Context, id string) (Plan, error) {
	var p Plan
	err := s.locked(ctx, func() error { var err error; p, err = s.load(id); return err })
	return p, err
}
func (s Store) valid(p Plan, a accounts.Account) error {
	if p.Scope.Account != a {
		return errors.New("plan account mismatch")
	}
	now := s.now()
	if now.Before(p.Scope.Created) || !now.Before(p.Scope.Expires) {
		return errors.New("plan expired or future-dated; create a fresh preview")
	}
	return nil
}
func (s Store) Approve(ctx context.Context, id string, a accounts.Account, confirm func(Plan) error) error {
	return s.locked(ctx, func() error {
		p, err := s.load(id)
		if err != nil {
			return err
		}
		if err = s.valid(p, a); err != nil {
			return err
		}
		if p.State != "prepared" || confirm == nil {
			return errors.New("plan is not awaiting human approval")
		}
		if err = confirm(p); err != nil {
			return errors.New("human approval not confirmed")
		}
		if err = s.valid(p, a); err != nil {
			return err
		}
		now := s.now()
		p.State = "approved"
		p.ApprovedAt = &now
		path, _ := s.path(id)
		return localstore.Write(path, p)
	})
}
func (s Store) Execute(ctx context.Context, id string, a accounts.Account, perform func(Plan) (json.RawMessage, error)) (Plan, error) {
	var result Plan
	err := s.locked(ctx, func() error {
		p, err := s.load(id)
		if err != nil {
			return err
		}
		if err = s.valid(p, a); err != nil {
			return err
		}
		if p.State != "approved" || p.ApprovedAt == nil || p.AttemptedAt != nil || perform == nil {
			return errors.New("plan not executable; never retry an attempted/unknown action")
		}
		now := s.now()
		p.State = "executing"
		p.AttemptedAt = &now
		path, _ := s.path(id)
		if err = localstore.Write(path, p); err != nil {
			return err
		}
		result = p // crash from here consumes this attempt
		response, callErr := perform(p)
		p.State = "unknown"
		if callErr == nil {
			p.State = "succeeded"
			p.Result = response
		}
		if err = localstore.Write(path, p); err != nil {
			return errors.New("cannot persist result; outcome unknown, do not retry")
		}
		result = p
		if callErr != nil {
			return errors.New("remote outcome unknown; plan consumed, reconcile before any new action")
		}
		return nil
	})
	return result, err
}
