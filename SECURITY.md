# Security Policy

Agent Radio is a solo-maintainer MIT project. Security reports are welcome, but they must be handled privately first.

## Supported versions

| Version | Security support |
| --- | --- |
| Latest GitHub Release | Supported |
| `master` | Supported for release-critical fixes before the next release |
| Older releases | Best effort only |

`master` is the protected release/default branch. Public contributions should target `develop`; security fixes may be coordinated privately by the maintainer before a public PR is opened.

## Reporting a vulnerability

Do not open a public GitHub issue for vulnerabilities.

Use GitHub private vulnerability reporting for this repository if it is enabled.
If private vulnerability reporting is not available, contact the maintainer
privately through their GitHub profile before opening any public issue.

When reporting, include:

- affected Agent Radio version or commit
- operating system and shell environment
- steps to reproduce
- expected impact
- whether the issue is already public

The maintainer will acknowledge reports as soon as practical and will make the final call on severity, fix scope, disclosure timing, and release timing.

## Local-first data model

Agent Radio is local-first. It does not require a hosted account or network sync, and it stores local state on the user's machine. Treat Agent Radio state and configuration as sensitive local plaintext, including:

- SQLite inbox/state files
- `~/.config/agent-radio/config.yaml`
- MCP/client configuration entries
- tmux session names, workspace paths, and message contents

Do not paste secrets into Agent Radio messages. Keep filesystem permissions appropriate for your machine and account.

## Public discussion

Use public issues for non-sensitive bugs, install problems, and feature requests. If a report might expose a secret, bypass behavior, unsafe command execution, local privilege concern, or private workspace information, use the private security contact instead.
