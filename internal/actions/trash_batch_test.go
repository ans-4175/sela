package actions

import (
	"fmt"
	"testing"
)

func TestTrashBatchBoundsAndSource(t *testing.T) {
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = fmt.Sprintf("id%d", i)
	}
	if err := ValidateTrashBatchIDs(ids); err != nil {
		t.Fatal(err)
	}
	if ValidateTrashBatchIDs(append(ids, "extra")) == nil || ValidateTrashBatchIDs([]string{"a", "a"}) == nil || ValidateTrashBatchIDs(nil) == nil {
		t.Fatal("invalid IDs accepted")
	}
	for _, test := range []struct {
		source, category string
		valid            bool
	}{
		{"inbox", "CATEGORY_PURCHASES", true}, {"inbox", "CATEGORY_SOCIAL", true}, {"inbox", "CATEGORY_UPDATES", true}, {"inbox", "CATEGORY_FORUMS", true}, {"inbox", "CATEGORY_PROMOTIONS", true}, {"inbox", "", false}, {"inbox", "CATEGORY_PERSONAL", false}, {"spam", "", true}, {"spam", "CATEGORY_SOCIAL", false}, {"trash", "", false},
	} {
		if (ValidateTrashSource(test.source, test.category) == nil) != test.valid {
			t.Fatal(test)
		}
	}
}
