package tmuxradio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
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

func TestWakePastesFullTextAndSubmits(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}

	ctx := context.Background()
	session := fmt.Sprintf("agent-radio-test-%d", time.Now().UnixNano())
	out := fmt.Sprintf("%s/out", t.TempDir())
	if err := StartDetached(ctx, session, []string{"bash", "-lc", "cat > " + shellQuote(out)}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Kill(ctx, session) })

	text := strings.Repeat("wake-message-", 40) + "tail"
	if err := Wake(ctx, session, text); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		got, _ := os.ReadFile(out)
		if string(got) == text+"\n" {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	got, _ := os.ReadFile(out)
	t.Fatalf("wake did not submit full text:\nwant %q\ngot  %q", text+"\n", string(got))
}
