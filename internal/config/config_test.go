package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
workspaces:
  - name: Test
    description: Test workspace
    root: /tmp
    capabilities: [testing]
    repositories:
      - id: agent-radio
        name: Agent Radio
        path: /tmp
        role: coordination-tool
        product: Example Product
        provides: [message-bus]
        capabilities: [go]
    sessions:
      - name: codex-test
        type: codex
        repo_id: agent-radio
        description: Test agent
        path: /tmp
        command: codex
        capabilities: [go]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Workspaces[0].Sessions[0].AgentID; got != "codex-test" {
		t.Fatalf("agent id = %q", got)
	}
	if got := cfg.Workspaces[0].Description; got != "Test workspace" {
		t.Fatalf("workspace description = %q", got)
	}
	if got := cfg.Workspaces[0].Sessions[0].Type; got != "codex" {
		t.Fatalf("session type = %q", got)
	}
	if got := cfg.Workspaces[0].Repositories[0].Role; got != "coordination-tool" {
		t.Fatalf("repository role = %q", got)
	}
	if got := cfg.Workspaces[0].Sessions[0].RepoID; got != "agent-radio" {
		t.Fatalf("session repo id = %q", got)
	}
}

func TestDuplicateSessionNamesRejected(t *testing.T) {
	cfg := Config{Workspaces: []Workspace{{
		Name: "Test",
		Sessions: []Session{
			{Name: "same", Path: "/tmp", Command: "bash"},
			{Name: "same", Path: "/tmp", Command: "bash"},
		},
	}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected duplicate session error")
	}
}

func TestUnknownSessionRepoIDRejected(t *testing.T) {
	cfg := Config{Workspaces: []Workspace{{
		Name: "Test",
		Repositories: []Repository{
			{ID: "known", Path: "/tmp"},
		},
		Sessions: []Session{
			{Name: "agent", RepoID: "missing", Path: "/tmp", Command: "bash"},
		},
	}}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected unknown repo_id error")
	}
}
