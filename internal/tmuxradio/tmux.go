package tmuxradio

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Session struct {
	Name string
}

func Sessions(ctx context.Context) ([]Session, error) {
	cmd := exec.CommandContext(ctx, "tmux", "list-sessions", "-F", "#{session_name}")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var sessions []Session
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			sessions = append(sessions, Session{Name: line})
		}
	}
	return sessions, nil
}

func HasSession(ctx context.Context, name string) bool {
	return exec.CommandContext(ctx, "tmux", "has-session", "-t", name).Run() == nil
}

func StartDetached(ctx context.Context, name string, command []string) error {
	if HasSession(ctx, name) {
		return nil
	}
	if len(command) == 0 {
		return fmt.Errorf("empty tmux command")
	}
	args := []string{"new-session", "-d", "-s", name}
	args = append(args, command...)
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "tmux", args...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("tmux new-session %s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func StartShell(ctx context.Context, name, dir, command, agentID string) error {
	if HasSession(ctx, name) {
		return nil
	}
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("empty tmux command")
	}
	shellCommand := fmt.Sprintf("cd %s && AGENT_RADIO_ID=%s %s",
		shellQuote(dir), shellQuote(agentID), command)
	return StartDetached(ctx, name, []string{"bash", "-lc", shellCommand})
}

func StartInteractiveShell(ctx context.Context, name, dir, command, agentID string) error {
	if HasSession(ctx, name) {
		return nil
	}
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("empty tmux command")
	}
	return StartDetached(ctx, name, []string{"bash", "-lc", interactiveShellCommand(dir, command, agentID)})
}

func interactiveShellCommand(dir, command, agentID string) string {
	return fmt.Sprintf("cd %s && export AGENT_RADIO_ID=%s; %s; rc=$?; printf '\\n[agent-radio] command exited with status %%s. Run %s to restart, or Ctrl+g to return to panel.\\n' \"$rc\"; exec bash -i",
		shellQuote(dir), shellQuote(agentID), command, command)
}

func DetachClients(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("empty session")
	}
	if !HasSession(ctx, name) {
		return fmt.Errorf("tmux session %q not found", name)
	}
	cmd := exec.CommandContext(ctx, "tmux", "detach-client", "-s", name)
	if err := cmd.Run(); err != nil {
		// tmux returns an error when no client is attached to the session. That
		// state is fine before opening a session from the panel.
		return nil
	}
	return nil
}

func Open(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("empty session")
	}
	if !HasSession(ctx, name) {
		return fmt.Errorf("tmux session %q not found", name)
	}
	if os.Getenv("TMUX") != "" {
		return exec.CommandContext(ctx, "tmux", "switch-client", "-t", name).Run()
	}
	return exec.CommandContext(ctx, "tmux", "attach-session", "-t", name).Run()
}

func Kill(ctx context.Context, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("empty session")
	}
	if !HasSession(ctx, name) {
		return nil
	}
	return exec.CommandContext(ctx, "tmux", "kill-session", "-t", name).Run()
}

func Capture(ctx context.Context, name string, lines int) (string, error) {
	if lines <= 0 {
		lines = 40
	}
	start := fmt.Sprintf("-%d", lines)
	cmd := exec.CommandContext(ctx, "tmux", "capture-pane", "-pt", name, "-S", start)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(out), "\n"), nil
}

func Wake(ctx context.Context, session, text string) error {
	if strings.TrimSpace(session) == "" {
		return fmt.Errorf("empty session")
	}
	if !HasSession(ctx, session) {
		return fmt.Errorf("tmux session %q not found", session)
	}
	clear := exec.CommandContext(ctx, "tmux", "send-keys", "-t", session, "C-u")
	if err := clear.Run(); err != nil {
		return err
	}
	typeCmd := exec.CommandContext(ctx, "tmux", "send-keys", "-t", session, "-l", text)
	if err := typeCmd.Run(); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(1200 * time.Millisecond):
	}
	return exec.CommandContext(ctx, "tmux", "send-keys", "-t", session, "Enter").Run()
}

func IsInfra(name string) bool {
	return strings.HasPrefix(name, "agent-radio-") || strings.HasPrefix(name, "ff-")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
