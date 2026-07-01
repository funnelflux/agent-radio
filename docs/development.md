# Development

## Requirements

- Go 1.24 or newer
- tmux
- bash
- curl for installer smoke tests

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

The release script writes binaries, shell helpers, and checksums under `dist/`.

It builds:

```text
dist/agent-radio-linux-amd64
dist/agent-radio-linux-arm64
dist/agent-radio-darwin-amd64
dist/agent-radio-darwin-arm64
dist/agent-radio-shell-helpers.sh
dist/checksums.txt
```

`VERSION` is injected into `agent-radio version` and MCP server metadata.

GitHub Actions can build releases from manual `v*` tag pushes or workflow
dispatches, but normal releases do not need manual tag creation. Merge a PR into
`master`; the auto-release-tag workflow tests `master`, creates the next semver
tag, builds the release artifacts, attests them, and publishes the GitHub
release.

Version bump rules:

- `release:major` or `semver:major` label: `v1.2.3` -> `v2.0.0`
- `release:minor` or `semver:minor` label: `v1.2.3` -> `v1.3.0`
- `release:patch` or `semver:patch` label: `v1.2.3` -> `v1.2.4`
- `release:none` label: skip tag creation
- PR title/body containing `BREAKING CHANGE` or `!:`: major
- PR title starting with `feat:` or `feat(scope):`: minor
- otherwise: patch

Before merging a release PR:

```bash
go test ./...
go build -o ./bin/agent-radio ./cmd/agent-radio
VERSION=v0.1.0 DIST_DIR=/tmp/agent-radio-dist scripts/build-release.sh
```

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
