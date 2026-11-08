package actions

import (
	"errors"
	"slices"
)

// Trash binds exactly one message and its reviewed metadata and label snapshot.
type Trash struct {
	ID           string   `json:"id"`
	ThreadID     string   `json:"thread_id"`
	InternalDate int64    `json:"internal_date_ms"`
	From         string   `json:"from"`
	Subject      string   `json:"subject"`
	Date         string   `json:"date"`
	Labels       []string `json:"labels"`
}

func (m Trash) Validate() error { return m.ValidateFor("inbox", "") }

// ValidateFor never relaxes the original single-message Inbox policy.
func (m Trash) ValidateFor(source, category string) error {
	if ValidateMessageIDs([]string{m.ID}) != nil || ValidateMessageIDs([]string{m.ThreadID}) != nil || m.InternalDate <= 0 || len(m.From) > 500 || len(m.Subject) > 500 || len(m.Date) > 500 || len(m.Labels) == 0 || len(m.Labels) > 100 {
		return errors.New("invalid trash snapshot")
	}
	for i, label := range m.Labels {
		if label == "" || len(label) > 128 || (i > 0 && m.Labels[i-1] >= label) {
			return errors.New("invalid trash labels")
		}
	}
	switch source {
	case "inbox":
		if !slices.Contains(m.Labels, "INBOX") || slices.Contains(m.Labels, "SPAM") {
			return errors.New("message is not eligible Inbox mail")
		}
		if category != "" && !slices.Contains(m.Labels, category) {
			return errors.New("message category changed or unavailable")
		}
	case "spam":
		if category != "" || !slices.Contains(m.Labels, "SPAM") || slices.Contains(m.Labels, "INBOX") {
			return errors.New("message is not eligible Spam mail")
		}
	default:
		return errors.New("unsupported trash source")
	}
	for _, protected := range []string{"TRASH", "DRAFT", "SENT", "STARRED", "IMPORTANT"} {
		if slices.Contains(m.Labels, protected) {
			return errors.New("message has a protected label")
		}
	}
	return nil
}
