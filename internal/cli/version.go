package cli

import (
	"runtime"
	"runtime/debug"
)

// Release builds inject these strings with -ldflags -X; local builds keep the dev version.
var version = "0.1.0-dev"
var commit = ""

func versionMetadata() map[string]any {
	revision, modified := commit, false
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" && revision == "" {
				revision = setting.Value
			}
			if setting.Key == "vcs.modified" {
				modified = setting.Value == "true"
			}
		}
	}
	if revision == "" {
		revision = "unknown"
	}
	return map[string]any{"version": version, "commit": revision, "modified": modified, "go_version": runtime.Version(), "platform": runtime.GOOS + "/" + runtime.GOARCH, "stage": "development; see --skill for validation scope"}
}
