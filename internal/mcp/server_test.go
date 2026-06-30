package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/funnelflux/agent-radio/internal/version"
)

func TestServeInitializeListAndMessageFlow(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
workspaces:
  - name: Agent Radio
    description: Local message bus
    root: /tmp
    capabilities: [coordination]
    repositories:
      - id: agent-radio
        name: Agent Radio
        description: Local tmux message bus
        path: /tmp
        role: coordination-tool
        product: FunnelFlux
        provides: [agent-message-bus]
        capabilities: [tmux]
    sessions:
      - name: codex-a
        repo_id: agent-radio
        type: codex
        description: sender
        path: /tmp
        command: codex
        capabilities: [go]
      - name: codex-b
        type: codex
        path: /tmp
        command: codex
  - name: Other Product
    root: /tmp
    repositories:
      - id: other-repo
        name: Other Repo
        description: unrelated repo
        path: /tmp
        role: other-role
    sessions:
      - name: codex-other
        repo_id: other-repo
        type: codex
        path: /tmp
        command: codex
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_RADIO_CONFIG", configPath)
	t.Setenv("AGENT_RADIO_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("AGENT_RADIO_ID", "codex-a")

	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"agent_radio_list_workspaces","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"agent_radio_list_repositories","arguments":{"role":"coordination-tool"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"agent_radio_context","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"agent_radio_list_agents","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"agent_radio_list_agents","arguments":{"scope":"all"}}}`,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"agent_radio_send","arguments":{"to":"codex-b","kind":"ASK","body":"proof"}}}`,
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"agent_radio_send","arguments":{"to":"codex-other","kind":"ASK","body":"leak"}}}`,
		`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"agent_radio_session_status","arguments":{"name":"codex-b"}}}`,
		`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"agent_radio_session_status","arguments":{"name":"codex-other"}}}`,
		"",
	}, "\n")

	var out bytes.Buffer
	if err := Serve(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}

	responses := readResponses(t, out.String())
	if len(responses) != 11 {
		t.Fatalf("response count = %d, want 11; output:\n%s", len(responses), out.String())
	}
	if responses[0]["error"] != nil {
		t.Fatalf("initialize error: %#v", responses[0]["error"])
	}
	if got := nested(t, responses[0], "result", "serverInfo", "version"); got != version.Version {
		t.Fatalf("initialize serverInfo version = %v, want %q", got, version.Version)
	}
	tools := nested(t, responses[1], "result", "tools")
	if tools == nil {
		t.Fatalf("tools/list missing tools: %#v", responses[1])
	}
	if !strings.Contains(toolText(t, responses[2]), `"name": "Agent Radio"`) {
		t.Fatalf("workspace result missing metadata: %s", toolText(t, responses[2]))
	}
	if !strings.Contains(toolText(t, responses[2]), `"repo_id": "agent-radio"`) {
		t.Fatalf("workspace result missing session repo link: %s", toolText(t, responses[2]))
	}
	if !strings.Contains(toolText(t, responses[3]), `"role": "coordination-tool"`) {
		t.Fatalf("repository result missing role: %s", toolText(t, responses[3]))
	}
	if !strings.Contains(toolText(t, responses[4]), `"workspace": "Agent Radio"`) {
		t.Fatalf("context result missing current workspace: %s", toolText(t, responses[4]))
	}
	if strings.Contains(toolText(t, responses[4]), `"id": "other-repo"`) {
		t.Fatalf("context result leaked unrelated repo: %s", toolText(t, responses[4]))
	}
	if strings.Contains(toolText(t, responses[5]), `"name": "codex-other"`) {
		t.Fatalf("default agent list leaked unrelated session: %s", toolText(t, responses[5]))
	}
	if nested(t, responses[6], "result", "isError") != true {
		t.Fatalf("scope all should be rejected: %#v", responses[6])
	}
	if !strings.Contains(toolText(t, responses[7]), `"body": "proof"`) {
		t.Fatalf("send result missing message: %s", toolText(t, responses[7]))
	}
	if nested(t, responses[8], "result", "isError") != true {
		t.Fatalf("cross-workspace send should be rejected: %#v", responses[8])
	}
	if !strings.Contains(toolText(t, responses[9]), `"name": "codex-b"`) {
		t.Fatalf("same-workspace session status missing session: %s", toolText(t, responses[9]))
	}
	if nested(t, responses[10], "result", "isError") != true {
		t.Fatalf("cross-workspace session status should be rejected: %#v", responses[10])
	}
}

func TestServeInboxIsBoundToCurrentAgent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`
workspaces:
  - name: Agent Radio
    root: /tmp
    sessions:
      - name: codex-a
        type: codex
        path: /tmp
        command: codex
      - name: codex-b
        type: codex
        path: /tmp
        command: codex
`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_RADIO_CONFIG", configPath)
	t.Setenv("AGENT_RADIO_STATE_DIR", filepath.Join(dir, "state"))
	t.Setenv("AGENT_RADIO_ID", "codex-a")

	var out bytes.Buffer
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"agent_radio_send","arguments":{"to":"codex-b","kind":"ASK","body":"proof"}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"agent_radio_inbox","arguments":{"peek":true}}}`,
		"",
	}, "\n")
	if err := Serve(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	responses := readResponses(t, out.String())
	if nested(t, responses[1], "result", "isError") == true {
		t.Fatalf("own inbox should not reject supported args: %#v", responses[1])
	}
	if strings.Contains(toolText(t, responses[1]), `"body": "proof"`) {
		t.Fatalf("sender inbox should not expose recipient message: %s", toolText(t, responses[1]))
	}

	t.Setenv("AGENT_RADIO_ID", "codex-b")
	out.Reset()
	input = strings.Join([]string{
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"agent_radio_inbox","arguments":{"peek":true}}}`,
		"",
	}, "\n")
	if err := Serve(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	responses = readResponses(t, out.String())
	if !strings.Contains(toolText(t, responses[0]), `"body": "proof"`) {
		t.Fatalf("own inbox missing message: %s", toolText(t, responses[0]))
	}
}

func readResponses(t *testing.T, output string) []map[string]any {
	t.Helper()
	var responses []map[string]any
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		var msg map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			t.Fatalf("bad response %q: %v", scanner.Text(), err)
		}
		responses = append(responses, msg)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return responses
}

func toJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func nested(t *testing.T, msg map[string]any, keys ...string) any {
	t.Helper()
	var cur any = msg
	for _, key := range keys {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[key]
	}
	return cur
}

func toolText(t *testing.T, msg map[string]any) string {
	t.Helper()
	content := nested(t, msg, "result", "content")
	items, ok := content.([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("missing tool content: %#v", msg)
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("bad tool content: %#v", msg)
	}
	text, ok := first["text"].(string)
	if !ok {
		t.Fatalf("bad text content: %#v", msg)
	}
	return text
}
