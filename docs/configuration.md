# Configuration

Agent Radio reads workspace configuration from:

```text
~/.config/agent-radio/config.yaml
```

Use `AGENT_RADIO_CONFIG` to point at another file while testing.

To create a starter file:

```bash
cd /path/to/project
agent-radio setup
```

`setup` writes a small example based on the current directory when no config
exists. It also installs Agent Radio as an MCP server for detected Codex, Claude
Code, and OpenCode clients unless you pass `--no-mcp`. It does not try to fully
model your project; edit the YAML afterward.

## Minimal Config

```yaml
workspaces:
  - name: My Project
    description: Local project agents
    root: ~/Dev/my-project
    color: cyan
    repositories:
      - id: my-project
        name: My Project
        path: ~/Dev/my-project
        role: Application repository
        description: Main application repository.
    sessions:
      - name: opencode-my-project
        type: opencode
        repo_id: my-project
        path: ~/Dev/my-project
        command: opencode
        agent_id: opencode-my-project
        color: blue
```

## Workspaces

A workspace is the default visibility boundary for discovery. Agents normally see
repositories and sessions in their own workspace first.

## Repositories

Repositories describe what code exists and what it is for. Keep this concise but
specific enough for another agent to choose the right repo.

Recommended fields:

- `id`
- `name`
- `path`
- `role`
- `description`

## Sessions

Sessions describe runnable tmux sessions. The session `name` is the tmux session
name and Agent Radio address.

Recommended fields:

- `name`
- `type`
- `repo_id`
- `path`
- `command`
- `agent_id`
- `color`

`repo_id` links a runnable session to a repository description.
