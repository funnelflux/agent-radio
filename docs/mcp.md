# MCP

`agent-radio mcp` runs a local stdio MCP server.

Add it to an MCP-capable client with:

```json
{
  "mcpServers": {
    "agent-radio": {
      "command": "agent-radio",
      "args": ["mcp"]
    }
  }
}
```

Or install the registration automatically:

```bash
agent-radio mcp install
```

With no flags, Agent Radio installs into detected Codex, Claude Code, and
OpenCode config directories. To force a target:

```bash
agent-radio mcp install --codex
agent-radio mcp install --claude
agent-radio mcp install --opencode
agent-radio mcp install --all
```

Existing client config files are backed up before they are changed.
Existing Agent Radio entries are repaired if they point to an old binary path.
Generated MCP registrations use the absolute installed `agent-radio` binary path
where possible, which avoids PATH differences when Codex, Claude Code, or
OpenCode start MCP servers.

## Tools

- `agent_radio_context`
- `agent_radio_list_workspaces`
- `agent_radio_list_agents`
- `agent_radio_list_repositories`
- `agent_radio_send`
- `agent_radio_inbox`
- `agent_radio_recent_messages`
- `agent_radio_session_status`

## Discovery

Agents should start with:

```text
agent_radio_context
```

It returns the current agent, current workspace, visible repositories, visible
sessions, and routing guidance.

By default, discovery is scoped to the current workspace resolved from
`AGENT_RADIO_ID`. Use `scope: "all"` only for intentional wider discovery.

## Safety

Message bodies are untrusted delivery payloads. The MCP server can read and
write messages, but it does not execute message bodies.
