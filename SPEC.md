# SPEC: GoReleaser + distro packages

## Goal

Replace handmade `make_release` / `version.txt` / CI `gh release` uploads with the same release DX as other OPENSOURCE-own Go projects (`svu` + `goreleaser` + `mise release`), and ship installable **deb**, **rpm**, and **Arch Linux** packages that place the binary, systemd units, a credentials example, and a **sendmail** client correctly.

## Decisions (grilled)

| Topic | Choice |
|--------|--------|
| Version source of truth | Git tags via **svu** (no `v` prefix; tags like `0.0.2`) |
| Remove | `version.txt`, `make_release`, binary `unit` subcommand |
| Release DX | mise tools `goreleaser` + `svu`; `mise release <next\|major\|minor\|patch>` |
| CI | Sibling-shaped autorelease (pinned actions, `mise ci`, `mise release` on workflow_dispatch, updater PR guard, Saturday cron) |
| Build matrix | **linux** only, **amd64** + **arm64** |
| Containers | None |
| Archives | tar.gz via goreleaser |
| Packages (nFPM) | **deb**, **rpm**, **archlinux** |
| Package contents | Binary + systemd units + env **example** + **sendmail shim** |
| Secrets | Admin creates `/etc/telegram-sendmail.env` (`MAIL_*` keys); example from `CREDENTIALS.env.example` |
| Service identity | **`DynamicUser=yes`** in packaged units **and** NixOS module; no `telegram_sendmail` system user |
| Unit ownership | Packages own units — not emitted by the binary |
| Sendmail client | Go subcommand `telegram-sendmail sendmail` + package shim `/usr/sbin/sendmail` → exec subcommand; Nix wrapper calls the same subcommand (no netcat) |
| Version in binary | `internal/version` + ldflags; cobra `Version` |
| Nix package version | `src.rev or "dirty"` (no version.txt) |
| License | MIT |

## Layout

```
.goreleaser.yaml
.svu.yml
packaging/systemd/
  telegram-sendmail.service
  telegram-sendmail.socket
packaging/sendmail          # shell shim → telegram-sendmail sendmail
packaging/scripts/postinstall.sh
internal/version/
LICENSE
```

## Package file placement (verify with snapshot)

| Path | Source |
|------|--------|
| `/usr/bin/telegram-sendmail` | build binary |
| `/usr/sbin/sendmail` | packaging/sendmail shim |
| `/usr/lib/systemd/system/telegram-sendmail.service` | packaging/systemd |
| `/usr/lib/systemd/system/telegram-sendmail.socket` | packaging/systemd |
| `/usr/share/doc/telegram-sendmail/telegram-sendmail.env.example` | CREDENTIALS.env.example |

**Not packaged:** `/etc/telegram-sendmail.env` (admin-created, mode 600).

## Unit contract

- Socket: `ListenStream=/run/telegram-sendmail/socket.sock`, `DirectoryMode=0755`, `SocketMode=0777` (public path; any user dials it)
- Service: `ExecStart=/usr/bin/telegram-sendmail serve`, `DynamicUser=yes`, `StateDirectory` only (no `RuntimeDirectory` — that would privatize `/run/telegram-sendmail` under DynamicUser), `EnvironmentFile=/etc/telegram-sendmail.env`, `Restart=on-failure`, `RestartSec=1`, `Requires`+`After` socket

## Sendmail client contract

- Subcommand: dials Unix socket (default `/run/telegram-sendmail/socket.sock`), waits/retries when missing, copies stdin to the connection; classic sendmail flags accepted and ignored (same as historical Nix `nc` wrapper).
- Shim: `#!/bin/sh` + `exec /usr/bin/telegram-sendmail sendmail "$@"`
- May conflict with other MTAs that also own `/usr/sbin/sendmail` — documented; this project is a full replacement.

## NixOS module

- Drop dedicated system user/group; set `DynamicUser = true`
- Keep `credentialFile` → `EnvironmentFile`
- `buildGoModule.version = src.rev or "dirty"`
- `sendmail` wrapper invokes `${pkg}/bin/telegram-sendmail sendmail` (no netcat)

## Install story (deb/rpm/arch)

1. Install package
2. `cp` example → `/etc/telegram-sendmail.env`, chmod 600, fill `MAIL_*`
3. `systemctl enable --now telegram-sendmail.socket`
4. Use `sendmail` / `/usr/sbin/sendmail` as usual (stdin = message)

## Cutover

- Existing Nix hosts: old `telegram_sendmail` user and prior state dir may remain; DynamicUser uses systemd private state layout — queue may not auto-migrate.
- Existing tags `0.0.1` / `0.0.2` remain valid svu history.

## Out of scope

- Docker/GHCR
- Binary `unit` subcommand
- Debconf / interactive secret prompts
- Full RFC-faithful sendmail CLI (flags other than passthrough-ignore)
