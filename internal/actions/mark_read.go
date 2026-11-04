package actions

import (
	"errors"
	"regexp"
	"strings"
)

const MaxMarkRead = 20

var messageID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

// MarkRead binds immutable message IDs, not a query or mutable thread membership.
type MarkReadMessage struct {
	ID      string `json:"id"`
	Subject string `json:"subject"`
	From    string `json:"from"`
}
type MarkRead struct {
	Messages []MarkReadMessage `json:"messages"`
}

func ValidateMessageIDs(ids []string) error {
	if len(ids) == 0 || len(ids) > MaxMarkRead {
		return errors.New("select 1-20 explicit message IDs")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !messageID.MatchString(id) || seen[id] {
			return errors.New("message IDs must be unique opaque IDs, not queries, threads or URLs")
		}
		seen[id] = true
	}
	return nil
}
func ParseMessageIDs(value string) ([]string, error) {
	ids := strings.Split(value, ",")
	return ids, ValidateMessageIDs(ids)
}
func (m MarkRead) IDs() []string {
	ids := make([]string, 0, len(m.Messages))
	for _, message := range m.Messages {
		ids = append(ids, message.ID)
	}
	return ids
}
func (m MarkRead) Validate() error { return ValidateMessageIDs(m.IDs()) }
