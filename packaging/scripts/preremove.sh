#!/bin/sh
# Conservative uninstall: stop/disable units; leave env and queue (SPEC).
set -e

if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
	systemctl disable --now telegram-sendmail.socket 2>/dev/null || true
	systemctl stop telegram-sendmail.service 2>/dev/null || true
	systemctl daemon-reload || true
fi
