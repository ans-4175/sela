package googleapi

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/ans-4175/sela/internal/actions"
	"google.golang.org/api/gmail/v1"
)

// MarkReadCandidate keeps only review metadata; message text never becomes an instruction.
func MarkReadCandidate(message *gmail.Message) (actions.MarkReadMessage, bool) {
	ref := actions.MarkReadMessage{ID: message.Id}
	if message.Payload != nil {
		for _, header := range message.Payload.Headers {
			switch strings.ToLower(header.Name) {
			case "subject":
				ref.Subject = header.Value
			case "from":
				ref.From = header.Value
			}
		}
	}
	return ref, slices.Contains(message.LabelIds, "UNREAD")
}

func (c *Clients) MarkReadPreview(ctx context.Context, ids []string) (actions.MarkRead, []string, error) {
	payload := actions.MarkRead{Messages: []actions.MarkReadMessage{}}
	skipped := []string{}
	if err := actions.ValidateMessageIDs(ids); err != nil {
		return payload, nil, err
	}
	for _, id := range ids {
		message, err := c.mail.Users.Messages.Get("me", id).Format("metadata").MetadataHeaders("From", "Subject").Fields("id,labelIds,payload/headers").Context(ctx).Do()
		if err != nil {
			return payload, nil, safe(err)
		}
		if message.Id != id {
			return payload, nil, errors.New("message metadata ID mismatch; nothing prepared")
		}
		ref, unread := MarkReadCandidate(message)
		if unread {
			payload.Messages = append(payload.Messages, ref)
		} else {
			skipped = append(skipped, id)
		}
	}
	return payload, skipped, nil
}

// One fixed mutation, followed by per-message read-back. No arbitrary labels or retry.
func (c *Clients) markRead(ctx context.Context, payload actions.MarkRead) (json.RawMessage, error) {
	if err := payload.Validate(); err != nil {
		return nil, err
	}
	ids := payload.IDs()
	if err := c.mail.Users.Messages.BatchModify("me", &gmail.BatchModifyMessagesRequest{Ids: ids, RemoveLabelIds: []string{"UNREAD"}}).Context(ctx).Do(); err != nil {
		return nil, safe(err)
	}
	for _, id := range ids {
		message, err := c.mail.Users.Messages.Get("me", id).Format("metadata").Fields("id,labelIds").Context(ctx).Do()
		if err != nil {
			return nil, safe(err)
		}
		if message.Id != id || slices.Contains(message.LabelIds, "UNREAD") {
			return nil, errors.New("mark-read result not verified; do not retry")
		}
	}
	return json.Marshal(map[string]any{"marked_read_ids": ids, "verified_read": true})
}
