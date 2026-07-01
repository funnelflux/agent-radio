# Changelog

All notable user-facing changes to Agent Radio will be documented here.

This project follows a simple changelog format. Dates use `YYYY-MM-DD`.

## Unreleased

### Added

- Open-source governance, contribution, security, issue, and pull request documentation.
- Security-oriented release hardening for generated local config files and router behavior.

### Changed

- Generated Agent Radio and MCP client config files now use private file permissions.
- Release-capable GitHub Actions are pinned to immutable action SHAs.
- Router wakeups are scoped to configured Agent Radio sessions instead of arbitrary tmux sessions.

### Fixed

- Removed avoidable private/company branding from public-facing metadata and test fixtures.

## Release notes

Tagged releases are published from the protected `master` branch. Public contribution work should target `develop` before release promotion.
