package cli

import (
	"embed"
	"errors"
	"io"
)

//go:embed skills/*.md
var skills embed.FS

func skill(out io.Writer, args []string) error {
	topic := "overview"
	if len(args) > 1 {
		return errors.New("--skill accepts at most one topic")
	}
	if len(args) == 1 {
		topic = args[0]
	}
	switch topic {
	case "overview", "gmail", "calendar", "workflows", "safety":
	default:
		return errors.New("unknown skill topic; choose gmail|calendar|workflows|safety")
	}
	b, e := skills.ReadFile("skills/" + topic + ".md")
	if e != nil {
		return errors.New("embedded skill unavailable")
	}
	_, e = out.Write(b)
	return e
}
