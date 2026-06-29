package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withEnv(t *testing.T, key, val string) {
	t.Helper()
	old, had := os.LookupEnv(key)
	if err := os.Setenv(key, val); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if had {
			_ = os.Setenv(key, old)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func TestSetupCreatesStarterConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	withEnv(t, "AGENT_RADIO_CONFIG", configPath)
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(oldWD) })

	var out bytes.Buffer
	if err := Run([]string{"setup", "--agent", "codex"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		"workspaces:",
		"command: \"codex\"",
		"agent_id: codex-",
		"agent-radio panel",
	} {
		if !strings.Contains(got+"\n"+out.String(), want) {
			t.Fatalf("setup output/config missing %q\nconfig:\n%s\nout:\n%s", want, got, out.String())
		}
	}
}

func TestSetupDoesNotOverwriteExistingConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	withEnv(t, "AGENT_RADIO_CONFIG", configPath)
	if err := os.WriteFile(configPath, []byte("sentinel\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := Run([]string{"setup", "--agent", "codex"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "sentinel\n" {
		t.Fatalf("setup overwrote existing config: %q", string(b))
	}
	if !strings.Contains(out.String(), "already exists") {
		t.Fatalf("setup did not report existing config: %s", out.String())
	}
}

func TestCLIFlowSendInboxDone(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	withEnv(t, "AGENT_RADIO_STATE_DIR", state)
	withEnv(t, "AGENT_RADIO_ID", "codex-a")

	var out bytes.Buffer
	if err := Run([]string{"ask", "codex-b", "can", "you", "check"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ASK codex-a -> codex-b") {
		t.Fatalf("unexpected ask output: %s", out.String())
	}

	withEnv(t, "AGENT_RADIO_ID", "codex-b")
	out.Reset()
	if err := Run([]string{"inbox"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "1) #1 ASK from codex-a") {
		t.Fatalf("unexpected inbox output: %s", out.String())
	}

	out.Reset()
	if err := Run([]string{"done", "1", "handled"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "DONE to codex-a re #1") {
		t.Fatalf("unexpected done output: %s", out.String())
	}

	withEnv(t, "AGENT_RADIO_ID", "codex-a")
	out.Reset()
	if err := Run([]string{"inbox", "--peek"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "DONE from codex-b") {
		t.Fatalf("sender did not receive done: %s", out.String())
	}
}

func TestDoctorReportsHealth(t *testing.T) {
	withEnv(t, "AGENT_RADIO_STATE_DIR", filepath.Join(t.TempDir(), "state"))
	withEnv(t, "AGENT_RADIO_ID", "agent-radio-test")

	var out bytes.Buffer
	if err := Run([]string{"doctor"}, &out, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "identity: agent-radio-test") {
		t.Fatalf("unexpected doctor output: %s", got)
	}
	for _, want := range []string{"router_session:", "session_count:", "schema: version 1"} {
		if !strings.Contains(got, want) {
			t.Fatalf("doctor missing %q in output: %s", want, got)
		}
	}
}
