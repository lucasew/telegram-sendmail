#!/bin/sh
# Seed secrets file and arm the socket unit (SPEC lifecycle: postinstall).
set -e

EXAMPLE=/usr/share/doc/telegram-sendmail/telegram-sendmail.env.example
ENVFILE=/etc/telegram-sendmail.env

if [ ! -e "$ENVFILE" ] && [ -f "$EXAMPLE" ]; then
	# Packager umask is often 022 (cp creates 0644). Set umask first so the
	# new secrets file is never world-readable between create and chmod.
	umask 077
	cp "$EXAMPLE" "$ENVFILE"
	chmod 0600 "$ENVFILE"
elif [ -f "$ENVFILE" ]; then
	# Never overwrite contents (SPEC). Still enforce mode: admins who
	# created the file with a loose umask leave real tokens world-readable.
	chmod 0600 "$ENVFILE" || true
fi

if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
	systemctl daemon-reload || true
	# Enable only — do not start (fresh env still has placeholders).
	systemctl enable telegram-sendmail.socket || true
fi
