# SPEC: GoReleaser + distro packages

## Goal

Replace handmade `make_release` / `version.txt` / CI `gh release` uploads with the same release DX as other OPENSOURCE-own Go projects (`svu` + `goreleaser` + `mise release`), and ship installable **deb**, **rpm**, and **Arch Linux** packages that place the binary, systemd units, a credentials example, and a **sendmail** client correctly.

**Primary consumers:** NixOS (module in-tree) and an Ubuntu VPS via `.deb`. RPM/Arch are first-class nFPM outputs but not a local QA matrix.

## Decisions (grilled)

| Topic | Choice |
|--------|--------|
| Version source of truth | Git tags via **svu** (no `v` prefix; tags like `0.0.2`) |
| Remove | `version.txt`, `make_release`, binary `unit` subcommand |
| Release DX | mise tools `goreleaser` + `svu`; `mise release <next\|major\|minor\|patch>` |
| CI | Sibling-shaped autorelease (pinned actions, `mise ci`, `mise release` on workflow_dispatch, updater PR guard, Saturday cron) |
| Quality bar | **`mise run ci`** only (includes packaging unit/shim tests via `go test`). No VM install matrix, lintian/rpmlint, or mandatory goreleaser snapshot in SPEC |
| Build matrix | **linux** only, **amd64** + **arm64** |
| Containers | None (Docker/GHCR out) |
| Archives | **Keep** tar.gz via goreleaser — **binary only** (not a full system install) |
| Packages (nFPM) | **deb**, **rpm**, **archlinux** |
| Package depends | **`systemd`** on all formats (`serve` requires socket activation). In `.goreleaser.yaml` the field is **`dependencies`** (GoReleaser), not nFPM standalone `depends` |
| Package contents | Binary + systemd units + env **example** + **sendmail shim** + no-op **`newaliases`** + LICENSE |
| Secrets / env | Required: `MAIL_TELEGRAM_TOKEN`, `MAIL_TELEGRAM_CHAT`. Other knobs optional (example file may list them; defaults not frozen here) |
| Env on disk | Postinstall **seeds** `/etc/telegram-sendmail.env` from the example **if missing**, mode **0600**. Never overwrite existing. Not shipped as a packaged config file that upgrades clobber |
| Service identity | **`DynamicUser=yes`** in packaged units **and** NixOS module; no `telegram_sendmail` system user |
| Legacy / cutover | **No legacy path.** Pre-DynamicUser user/state is unsupported; no migration. Old tags remain valid svu history only |
| Unit ownership | Packages own units — not emitted by the binary |
| Sendmail client | Go subcommand `telegram-sendmail sendmail` + package shim `/usr/sbin/sendmail` → exec subcommand; Nix wrapper calls the same subcommand (no netcat) |
| Wire protocol | **Fire-and-forget.** Client success = socket wait + dial + copy stdin. Server owns queue/Telegram failures. Server may write `OK`/errors on the socket; client does not require an ack |
| Socket threat model | **Public by design** (`SocketMode=0777`, world-traversable parent). Trust boundary: every local account may enqueue messages that use the configured bot/chat |
| Queue | **Infinite retry** until Telegram send succeeds. Misconfiguration can grow `StateDirectory` without bound; ops fix env or wipe state. No quarantine |
| Sendmail CLI | All classic flags **and** positional recipients **ignored**. One destination: configured Telegram chat. Not a real MTA router. No sysexits mapping required |
| Version in binary | `internal/version` + ldflags; cobra `Version` |
| Nix package version | `src.rev or "dirty"` (no version.txt) |
| License | MIT |

### Package manager MTA metadata (nFPM)

Send-only drop-in (msmtp-mta / nullmailer shape), not a full MTA. If Fedora alternatives matter later, open an issue.

| Format | Metadata |
|--------|----------|
| **deb** | `Provides` / `Conflicts` / `Replaces`: `mail-transport-agent`. Ship no-op **`newaliases`** (Debian Policy for MTA providers). |
| **archlinux** | `provides: smtp-forwarder`; `conflicts: smtp-forwarder, smtp-server` |
| **rpm** | `Provides: MTA, smtpdaemon, /usr/sbin/sendmail`. Hard-own `/usr/sbin/sendmail` shim. **No** Fedora-style `alternatives` scripts |

## Layout

```
.goreleaser.yaml
.svu.yml
packaging/systemd/
  telegram-sendmail.service
  telegram-sendmail.socket
packaging/sendmail          # shell shim → telegram-sendmail sendmail
packaging/newaliases        # no-op (deb policy; ship on all formats for simplicity)
packaging/scripts/postinstall.sh
packaging/scripts/preremove.sh
internal/version/
LICENSE
```

## Package file placement

| Path | Source |
|------|--------|
| `/usr/bin/telegram-sendmail` | build binary |
| `/usr/sbin/sendmail` | packaging/sendmail shim |
| `/usr/sbin/newaliases` | packaging/newaliases no-op |
| `/usr/lib/systemd/system/telegram-sendmail.service` | packaging/systemd |
| `/usr/lib/systemd/system/telegram-sendmail.socket` | packaging/systemd |
| `/usr/share/doc/telegram-sendmail/telegram-sendmail.env.example` | CREDENTIALS.env.example |
| `/usr/share/doc/telegram-sendmail/LICENSE` | LICENSE |

**Created by postinstall (not a package payload):** `/etc/telegram-sendmail.env` if absent, mode `0600`.

**Not packaged as replaceable config:** live secrets file (admin-owned after seed).

## Unit contract

- Socket: `ListenStream=/run/telegram-sendmail/socket.sock`, `DirectoryMode=0755`, `SocketMode=0777` (public by design; any local user dials it)
- Service: `ExecStart=/usr/bin/telegram-sendmail serve`, `DynamicUser=yes`, `StateDirectory` only (no `RuntimeDirectory` — that would privatize `/run/telegram-sendmail` under DynamicUser), `EnvironmentFile=/etc/telegram-sendmail.env`, `Restart=on-failure`, `RestartSec=1`, `Requires`+`After` socket

## Sendmail client contract

- Subcommand: dials Unix socket (default `/run/telegram-sendmail/socket.sock`), waits/retries when missing, copies stdin to the connection (**fire-and-forget**; no ack required).
- Classic sendmail flags and positional recipients are accepted and ignored. Destination is always the env-configured Telegram chat.
- Exit 0 means wait + dial + stdin copy succeeded, **not** that Telegram delivery or queueing succeeded.
- Shim: `#!/bin/sh` + `exec /usr/bin/telegram-sendmail sendmail "$@"`
- Owning `/usr/sbin/sendmail` conflicts with other MTAs — this project is a full replacement on hosts that only need Telegram delivery.

## Lifecycle scripts

### postinstall

When systemd is live (`systemctl` present and `/run/systemd/system` exists):

1. If `/etc/telegram-sendmail.env` is **missing**, copy the packaged example to that path and `chmod 0600` (do **not** overwrite if present).
2. `systemctl daemon-reload` (best-effort; ignore failure).
3. `systemctl enable telegram-sendmail.socket` — **enable only, do not start** (`--now` forbidden). Placeholders in a fresh env would only produce restart noise.

After install: admin edits `MAIL_TELEGRAM_TOKEN` / `MAIL_TELEGRAM_CHAT`, then `systemctl start telegram-sendmail.socket` (or reboot).

### preremove / uninstall (conservative)

1. `systemctl disable --now telegram-sendmail.socket` (and stop the service if up) when systemd is live.
2. `daemon-reload` after unit removal as appropriate for the packager.
3. **Do not** delete `/etc/telegram-sendmail.env`.
4. **Do not** wipe DynamicUser state / queue. Admin cleans manually if desired.

No deb `postrm purge` wipe of secrets/state unless requested later.

## NixOS module

- `DynamicUser = true` only (no dedicated system user/group)
- `credentialFile` → `EnvironmentFile`
- `buildGoModule.version = src.rev or "dirty"`
- `sendmail` wrapper invokes `${pkg}/bin/telegram-sendmail sendmail` (no netcat)
- No migration from any pre-DynamicUser layout

## Install story (deb/rpm/arch)

1. Install package (depends on systemd; may replace another MTA via virtual provides/conflicts).
2. Confirm `/etc/telegram-sendmail.env` was seeded; edit real `MAIL_TELEGRAM_TOKEN` and `MAIL_TELEGRAM_CHAT`.
3. `systemctl start telegram-sendmail.socket` (socket already **enabled** by postinstall).
4. Use `sendmail` / `/usr/sbin/sendmail` as usual (stdin = message).

## Out of scope

- Docker / GHCR
- Binary `unit` subcommand
- Debconf / interactive secret prompts
- Full RFC-faithful sendmail CLI (routing, recipients, sysexits)
- Fedora/RHEL `alternatives` MTA integration (hard-own sendmail + Provides only)
- Queue quarantine / bounded retry / poison-pill drop
- VM or multi-distro install matrix in CI
- Legacy `telegram_sendmail` user or queue migration
