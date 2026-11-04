//go:build windows

package cli

import (
	"errors"
	"github.com/ans-4175/sela/internal/plans"
)

func ConfirmTerminal(plans.Plan) error {
	return errors.New("Windows approval is not enabled until private storage/console ACL runtime tests pass")
}
