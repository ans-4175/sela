package actions

import (
	"errors"
	"regexp"
	"strings"
)

const MaxTrashBatch = 100

var batchID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`)

type TrashBatch struct {
	Source   string  `json:"source"`
	Category string  `json:"category,omitempty"`
	Messages []Trash `json:"messages"`
}

func ValidateTrashBatchIDs(ids []string) error {
	if len(ids) < 1 || len(ids) > MaxTrashBatch {
		return errors.New("select 1-100 explicit message IDs")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !batchID.MatchString(id) || seen[id] {
			return errors.New("unique opaque message IDs required")
		}
		seen[id] = true
	}
	return nil
}
func ParseTrashBatchIDs(value string) ([]string, error) {
	ids := strings.Split(value, ",")
	return ids, ValidateTrashBatchIDs(ids)
}
func ValidateTrashSource(source, category string) error {
	if source == "spam" && category == "" {
		return nil
	}
	if source == "inbox" {
		switch category {
		case "CATEGORY_PURCHASES", "CATEGORY_SOCIAL", "CATEGORY_UPDATES", "CATEGORY_FORUMS", "CATEGORY_PROMOTIONS":
			return nil
		}
	}
	return errors.New("select one supported Inbox category or spam without category")
}
func (b TrashBatch) Validate() error {
	if err := ValidateTrashSource(b.Source, b.Category); err != nil {
		return err
	}
	ids := make([]string, 0, len(b.Messages))
	for _, m := range b.Messages {
		ids = append(ids, m.ID)
		if err := m.ValidateFor(b.Source, b.Category); err != nil {
			return err
		}
	}
	return ValidateTrashBatchIDs(ids)
}
