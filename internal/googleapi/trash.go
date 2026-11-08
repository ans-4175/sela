package googleapi

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/ans-4175/sela/internal/actions"
	"google.golang.org/api/gmail/v1"
)

// trashSnapshot rejects ambiguous or oversized review metadata rather than silently truncating it.
func trashSnapshot(m *gmail.Message, id string) (actions.Trash, error) {
	var out actions.Trash
	if m == nil || m.Id != id || m.Payload == nil {
		return out, errors.New("message metadata incomplete or mismatched; nothing prepared")
	}
	out = actions.Trash{ID: m.Id, ThreadID: m.ThreadId, InternalDate: m.InternalDate, Labels: slices.Clone(m.LabelIds)}
	slices.Sort(out.Labels)
	seen := map[string]bool{}
	for _, h := range m.Payload.Headers {
		if h == nil {
			return actions.Trash{}, errors.New("invalid message header")
		}
		name := strings.ToLower(h.Name)
		if name != "from" && name != "subject" && name != "date" {
			continue
		}
		if seen[name] || !utf8.ValidString(h.Value) || len(h.Value) > 500 {
			return actions.Trash{}, errors.New("ambiguous or oversized review header")
		}
		seen[name] = true
		switch name {
		case "from":
			out.From = h.Value
		case "subject":
			out.Subject = h.Value
		case "date":
			out.Date = h.Value
		}
	}
	if !seen["from"] || !seen["subject"] || !seen["date"] {
		return actions.Trash{}, errors.New("missing review metadata")
	}
	return out, nil
}

func (c *Clients) TrashPreview(ctx context.Context, id string) (actions.Trash, error) {
	if err := actions.ValidateMessageIDs([]string{id}); err != nil {
		return actions.Trash{}, err
	}
	m, err := c.readMail.Users.Messages.Get("me", id).Format("metadata").MetadataHeaders("From", "Subject", "Date").Fields("id,threadId,internalDate,labelIds,payload/headers").Context(ctx).Do()
	if err != nil {
		return actions.Trash{}, safe(err)
	}
	snapshot, err := trashSnapshot(m, id)
	if err != nil {
		return actions.Trash{}, err
	}
	if err = snapshot.Validate(); err != nil {
		return actions.Trash{}, err
	}
	return snapshot, nil
}

// One exact-message write only. An observed postcondition is not an atomicity guarantee.
func (c *Clients) trash(ctx context.Context, snapshot actions.Trash) (json.RawMessage, error) {
	return c.trashFor(ctx, snapshot, "inbox", "")
}

func (c *Clients) trashFor(ctx context.Context, snapshot actions.Trash, source, category string) (json.RawMessage, error) {
	if err := snapshot.ValidateFor(source, category); err != nil {
		return nil, err
	}
	fresh, err := c.trashPreviewFor(ctx, snapshot.ID, source, category)
	if err != nil || !reflect.DeepEqual(fresh, snapshot) {
		return nil, errors.New("trash eligibility or reviewed metadata changed; no write attempted")
	}
	_, err = c.mail.Users.Messages.Trash("me", snapshot.ID).Context(ctx).Do()
	if err != nil {
		return nil, safe(err)
	}
	m, err := c.readMail.Users.Messages.Get("me", snapshot.ID).Format("metadata").Fields("id,labelIds").Context(ctx).Do()
	if err != nil || m == nil || m.Id != snapshot.ID || !slices.Contains(m.LabelIds, "TRASH") || slices.Contains(m.LabelIds, "INBOX") || slices.Contains(m.LabelIds, "SPAM") {
		return nil, errors.New("trash result not verified; do not retry")
	}
	return json.Marshal(map[string]any{"message_id": snapshot.ID, "status": "verified_trashed", "notice": "Observed Trash and absence from Inbox; concurrent changes cannot be ruled out."})
}
