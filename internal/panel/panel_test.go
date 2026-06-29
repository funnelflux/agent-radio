package panel

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPadPlainDoesNotCropExactWidthStyledRows(t *testing.T) {
	line := selectedRow("Agent Radio                       1/2   0", 44)
	if got := lipgloss.Width(line); got != 44 {
		t.Fatalf("selected row width = %d, want 44", got)
	}

	padded := padPlain(line, 44)
	if !strings.Contains(padded, "1/2") || !strings.Contains(padded, "0") {
		t.Fatalf("selected workspace counters were cropped: %q", padded)
	}
}

func TestSelectedSessionRowKeepsActivityColumn(t *testing.T) {
	line := selectedRow("codex-agent-radio Codex     alive    0    idle      ", 54)
	if got := lipgloss.Width(line); got != 54 {
		t.Fatalf("selected row width = %d, want 54", got)
	}

	padded := padPlain(line, 54)
	if !strings.Contains(padded, "alive") || !strings.Contains(padded, "idle") {
		t.Fatalf("selected session columns were cropped: %q", padded)
	}
}

func TestTmuxOpenScriptBindsReturnToPanelWhenInsideTmux(t *testing.T) {
	script := tmuxOpenScript("switch-client", "opencode-runtime")

	for _, want := range []string{
		"panel=$(tmux display-message -p '#S'",
		"tmux bind-key -n C-g switch-client -t \"$panel\"",
		"if [ \"$panel\" != 'opencode-runtime' ]",
		"tmux detach-client -s 'opencode-runtime'",
		"tmux switch-client -t 'opencode-runtime'",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("tmux open script missing %q: %s", want, script)
		}
	}
}

func TestTmuxOpenScriptBindsReturnToDetachWhenOutsideTmux(t *testing.T) {
	script := tmuxOpenScript("attach-session", "opencode-runtime")

	for _, want := range []string{
		"tmux bind-key -n C-g detach-client",
		"tmux detach-client -s 'opencode-runtime'",
		"tmux attach-session -t 'opencode-runtime'",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("tmux attach script missing %q: %s", want, script)
		}
	}
}
