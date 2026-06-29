# Development

## Requirements

- Go
- tmux
- bash

## Build

```bash
go build -o ./bin/agent-radio ./cmd/agent-radio
```

## Test

```bash
go test ./...
```

## Release Artifacts

```bash
VERSION=v0.1.0 scripts/build-release.sh
```

The release script writes binaries and checksums under `dist/`.

It builds:

```text
dist/agent-radio-linux-amd64
dist/agent-radio-linux-arm64
dist/agent-radio-darwin-amd64
dist/agent-radio-darwin-arm64
dist/checksums.txt
```

GitHub Actions runs the same release build for tags matching `v*` and uploads
those files to the GitHub release.

## Manual Smoke

Starter config:

```bash
AGENT_RADIO_CONFIG=/tmp/agent-radio-config.yaml ./bin/agent-radio setup --agent bash
```

```bash
AGENT_RADIO_STATE_DIR=/tmp/agent-radio-proof AGENT_RADIO_ID=agent-a ./bin/agent-radio ask agent-b "proof"
AGENT_RADIO_STATE_DIR=/tmp/agent-radio-proof AGENT_RADIO_ID=agent-b ./bin/agent-radio inbox
AGENT_RADIO_STATE_DIR=/tmp/agent-radio-proof AGENT_RADIO_ID=agent-b ./bin/agent-radio reply 1 "reply"
AGENT_RADIO_STATE_DIR=/tmp/agent-radio-proof AGENT_RADIO_ID=agent-a ./bin/agent-radio inbox --peek
```

For panel changes:

```bash
./bin/agent-radio panel
```
