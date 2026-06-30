# Contributing

Thanks for helping improve Agent Radio.

Agent Radio is an MIT-licensed, solo-maintainer project. Contributions are welcome, but the maintainer makes final decisions on scope, design, merge timing, and releases.

## Branches

- `master` is the protected release/default branch.
- `develop` is the public contribution branch.
- Open pull requests against `develop` unless the maintainer asks otherwise.

## Workflow

1. Check existing issues and pull requests before starting.
2. For larger changes, open an issue first so scope can be agreed.
3. Create a focused branch from `develop`.
4. Keep changes small and reviewable.
5. Update user-facing docs when behavior changes.
6. Run the narrow checks that cover your change before opening a PR.
7. Open a PR to `develop` and complete the checklist.


## Commit and PR titles

Use Conventional Commit-style PR titles because release automation uses the
merged PR title, body, and labels to choose the next tag:

- `fix: ...`, `docs: ...`, `chore: ...`, and similar changes default to patch
- `feat: ...` triggers a minor release
- `type!: ...` or `BREAKING CHANGE:` triggers a major release
- `release:none` skips release tagging for docs-only or internal-only PRs

If the inferred bump would be wrong, ask the maintainer to apply one explicit
label: `release:major`, `release:minor`, `release:patch`, or `release:none`.

## Development

See `docs/development.md` for build, test, release-artifact, and manual smoke-test commands.

Useful local checks:

- `go test ./...` for Go changes
- manual `agent-radio setup`, `agent-radio up`, and `agent-radio panel` smoke tests when changing runtime behavior
- install-script smoke tests when changing installation docs or installer behavior

## Review model

The maintainer reviews for correctness, local-first safety, maintainability, and fit with the project direction. A PR may be closed even if the code works when it adds maintenance cost, changes the product shape, or is better handled differently.

Please be patient with review timing. This project is maintained by one person.

## Security

Do not report vulnerabilities in public issues or PRs. Follow `SECURITY.md` instead.

Agent Radio stores local plaintext state. Avoid adding features that send local state, messages, paths, or config to remote services unless the design has been discussed and explicitly accepted.
