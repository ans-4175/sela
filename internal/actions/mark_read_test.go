package actions

import (
	"fmt"
	"strings"
	"testing"
)

func TestMarkReadExactBoundedIDs(t *testing.T) {
	ids, err := ParseMessageIDs("alpha,beta")
	if err != nil || len(ids) != 2 {
		t.Fatal(err)
	}
	for _, value := range []string{"", "alpha,alpha", "alpha, beta", "https://example.test/", "in:inbox", "alpha,"} {
		if _, err := ParseMessageIDs(value); err == nil {
			t.Fatal("invalid ID input accepted", value)
		}
	}
	many := []string{}
	for i := 0; i <= MaxMarkRead; i++ {
		many = append(many, fmt.Sprintf("id%d", i))
	}
	if _, err := ParseMessageIDs(strings.Join(many, ",")); err == nil {
		t.Fatal("unbounded selection accepted")
	}
}
