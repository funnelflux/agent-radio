package tmuxradio

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Session struct {
	Name         string
	PaneTitle    string
	PaneCommand  string
	LastActivity time.Time
}

func Sessions(ctx context.Context) ([]Session, error) {
	format := strings.Join([]string{
		"#{session_name}",
		"#{pane_title}",
		"#{pane_current_command}",
		"#{window_activity}",
	}, "\t")
	cmd := exec.CommandContext(ctx, "tmux", "list-panes", "-a", "-F", format)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var sessions []Session
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) == 0 {
			continue
		}
		name := strings.TrimSpace(fields[0])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		session := Session{Name: name}
		if len(fields) > 1 {
			session.PaneTitle = strings.TrimSpace(fields[1])
		}
		if len(fields) > 2 {
			session.PaneCommand = strings.TrimSpace(fields[2])
		}
		if len(fields) > 3 {
			if unix, err := strconv.ParseInt(strings.TrimSpace(fields[3]), 10, 64); err == nil && unix > 0 {
				session.LastActivity = time.Unix(unix, 0)
			}
		}
		sessions = append(sessions, session)
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

func CurrentSession(ctx context.Context) (string, error) {
	target := strings.TrimSpace(os.Getenv("TMUX_PANE"))
	if target == "" {
		if strings.TrimSpace(os.Getenv("TMUX")) == "" {
			return "", fmt.Errorf("not inside tmux")
		}
		target = "."
	}
	out, err := exec.CommandContext(ctx, "tmux", "display-message", "-p", "-t", target, "#S").Output()
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(string(out))
	if name == "" {
		return "", fmt.Errorf("empty tmux session")
	}
	return name, nil
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
	buffer := fmt.Sprintf("agent-radio-wake-%d", time.Now().UnixNano())
	set := exec.CommandContext(ctx, "tmux", "set-buffer", "-b", buffer, text)
	if err := set.Run(); err != nil {
		return err
	}
	paste := exec.CommandContext(ctx, "tmux", "paste-buffer", "-d", "-b", buffer, "-t", session)
	if err := paste.Run(); err != nil {
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
	return strings.HasPrefix(name, "agent-radio-")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
