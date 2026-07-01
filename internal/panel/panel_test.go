package panel

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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

func TestPanelErrorStatusRendersInNavNotFooter(t *testing.T) {
	m := &model{width: 120, status: `error: session "codex-api-docs-zudoku" is not running`}

	if got := m.navBar(); !strings.Contains(got, m.status) {
		t.Fatalf("navBar missing error status: %q", got)
	}
	if got := m.footer(); strings.Contains(got, m.status) {
		t.Fatalf("footer should not render error status: %q", got)
	}
}

func TestWorkspaceFocusDoesNotActOnSession(t *testing.T) {
	m := &model{width: 120, tab: tabWorkspaces, focus: 0}

	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("enter on workspace focus should not open a session")
	}
	if m.focus != 1 {
		t.Fatalf("enter should move focus to sessions, got %d", m.focus)
	}

	m.focus = 0
	_, cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd != nil {
		t.Fatal("s on workspace focus should not start a session")
	}

	_, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if m.confirm != "" {
		t.Fatalf("k on workspace focus should not arm session kill, got %q", m.confirm)
	}
}

func TestClassifyActivityFromPaneTitle(t *testing.T) {
	now := time.Unix(100, 0)
	tests := []struct {
		name string
		in   sessionState
		want string
	}{
		{
			name: "stopped",
			in:   sessionState{},
			want: "idle",
		},
		{
			name: "spinner title",
			in:   sessionState{alive: true, paneTitle: "⠋ flux-saas-runtime", lastActivity: now.Add(-time.Minute)},
			want: "work",
		},
		{
			name: "approval text",
			in:   sessionState{alive: true, paneTitle: "codex", snapshot: "Allow command? approve / deny"},
			want: "input",
		},
		{
			name: "codex idle title",
			in:   sessionState{alive: true, paneTitle: "flux-saas-runtime", paneCommand: "bash", lastActivity: now.Add(-time.Minute)},
			want: "wait",
		},
		{
			name: "opencode generic title",
			in:   sessionState{alive: true, paneTitle: "OpenCode", paneCommand: "bash", lastActivity: now.Add(-time.Minute)},
			want: "idle",
		},
		{
			name: "recent tmux activity",
			in:   sessionState{alive: true, paneTitle: "bash", paneCommand: "bash", lastActivity: now.Add(-5 * time.Second)},
			want: "work",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyActivity(tt.in, now); got != tt.want {
				t.Fatalf("classifyActivity() = %q, want %q", got, tt.want)
			}
		})
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
