package cli

import (
	"runtime"
	"testing"
)

func TestReleaseVersionMetadata(t *testing.T) {
	oldVersion, oldCommit := version, commit
	t.Cleanup(func() { version, commit = oldVersion, oldCommit })
	version = "v0.2.0-alpha.1"
	commit = "0123456789012345678901234567890123456789"
	m := versionMetadata()
	if m["version"] != version || m["commit"] != commit || m["platform"] != runtime.GOOS+"/"+runtime.GOARCH || m["go_version"] != runtime.Version() {
		t.Fatal("release identity mismatch", m)
	}
}
