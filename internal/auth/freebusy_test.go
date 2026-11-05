package auth

import (
	"golang.org/x/oauth2"
	"reflect"
	"strings"
	"testing"
)

func TestFreeBusyScopeIndependentOptInAndMissingConsentRefused(t *testing.T) {
	for _, write := range []bool{false, true} {
		for _, modify := range []bool{false, true} {
			base := ProfileScopes(write, modify)
			if !reflect.DeepEqual(base, ConnectionScopes(write, modify, false)) {
				t.Fatal("legacy profiles changed")
			}
			expanded := ConnectionScopes(write, modify, true)
			if len(expanded) != len(base)+1 || expanded[len(base)] != CalendarFreeBusyScope {
				t.Fatal(expanded)
			}
			token := (&oauth2.Token{}).WithExtra(map[string]any{"scope": strings.Join(base, " ")})
			if ValidateScopes(token, expanded) == nil {
				t.Fatal("availability without explicit consent")
			}
			token = token.WithExtra(map[string]any{"scope": strings.Join(expanded, " ")})
			if err := ValidateScopes(token, expanded); err != nil {
				t.Fatal(err)
			}
		}
	}
}
