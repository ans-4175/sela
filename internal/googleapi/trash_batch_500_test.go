package googleapi

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestTrashBatchPreview500AndCancellation(t *testing.T) {
	ids := make([]string, 500)
	for i := range ids {
		ids[i] = fmt.Sprintf("id%03d", i)
	}
	for _, cancelAt := range []int{0, 250} {
		ctx, cancel := context.WithCancel(context.Background())
		reads := 0
		c, _ := NewWithHTTP(ctx, &http.Client{Transport: roundtrip(func(r *http.Request) (*http.Response, error) {
			if r.Method != "GET" {
				t.Fatal("unexpected mutation")
			}
			reads++
			if cancelAt > 0 && reads == cancelAt {
				cancel()
				return nil, context.Canceled
			}
			id := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
			return reply(200, strings.Replace(fixtureTrash("INBOX", "CATEGORY_PROMOTIONS"), `"id":"alpha"`, `"id":"`+id+`"`, 1)), nil
		})})
		batch, _, err := c.TrashBatchPreview(ctx, ids, "inbox", "CATEGORY_PROMOTIONS")
		if cancelAt == 0 {
			if err != nil || len(batch.Messages) != 500 || reads != 500 {
				t.Fatal(len(batch.Messages), reads, err)
			}
		} else if err == nil || reads != cancelAt {
			t.Fatal(reads, err)
		}
		cancel()
	}
}
