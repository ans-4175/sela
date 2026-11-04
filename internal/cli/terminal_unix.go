//go:build darwin || linux

package cli

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/ans-4175/sela/internal/plans"
	"io"
	"os"
	"strings"
)

// No stdin/flag approval fallback. A controlling terminal is mandatory.
func ConfirmTerminal(p plans.Plan) error {
	tty, e := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if e != nil {
		return errors.New("human controlling terminal required")
	}
	defer tty.Close()
	info, e := tty.Stat()
	if e != nil || info.Mode()&os.ModeCharDevice == 0 {
		return errors.New("human terminal unavailable")
	}
	if e = encode(tty, p); e != nil {
		return e
	}
	fmt.Fprintf(tty, "\nReview exact account/payload/recipients/notifications above. Type APPROVE %s to approve only this plan (expiry unchanged):\n", p.Hash)
	line, e := bufio.NewReader(io.LimitReader(tty, 128)).ReadString('\n')
	if e != nil || strings.TrimSpace(line) != "APPROVE "+p.Hash {
		return errors.New("approval phrase did not match")
	}
	return nil
}
