#!/bin/sh
# Seed secrets file and arm the socket unit (SPEC lifecycle: postinstall).
set -e

EXAMPLE=/usr/share/doc/telegram-sendmail/telegram-sendmail.env.example
ENVFILE=/etc/telegram-sendmail.env

if [ ! -e "$ENVFILE" ] && [ -f "$EXAMPLE" ]; then
	cp "$EXAMPLE" "$ENVFILE"
	chmod 0600 "$ENVFILE"
fi

if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
	systemctl daemon-reload || true
	# Enable only — do not start (fresh env still has placeholders).
	systemctl enable telegram-sendmail.socket || true
fi
