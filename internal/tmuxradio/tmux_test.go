package tmuxradio

import (
	"strings"
	"testing"
)

func TestInteractiveShellCommandKeepsPaneAlive(t *testing.T) {
	cmd := interactiveShellCommand("/tmp/repo", "opencode", "opencode-repo")

	for _, want := range []string{
		"export AGENT_RADIO_ID='opencode-repo'",
		"opencode; rc=$?",
		"Ctrl+g to return to panel",
		"exec bash -i",
	} {
		if !strings.Contains(cmd, want) {
			t.Fatalf("interactive shell command missing %q: %s", want, cmd)
		}
	}
}
