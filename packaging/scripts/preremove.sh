#!/bin/sh
# Conservative uninstall: stop/disable units; leave env and queue (SPEC).
# Skip on package *upgrade* so deb/rpm scriptlets do not tear down a live
# socket mid-replace. postinstall only enables (does not start), so an upgrade
# path that disable --now's here would leave the service stopped until reboot.
set -e

# Scriptlet args:
#   deb: remove | upgrade | deconfigure | failed-upgrade
#   rpm: 0 = erase, >0 = upgrade (packages remaining)
#   arch/local: often no args → treat as uninstall
case "${1:-}" in
	upgrade|failed-upgrade|[1-9]|[1-9][0-9]*)
		exit 0
		;;
esac

if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
	systemctl disable --now telegram-sendmail.socket 2>/dev/null || true
	systemctl stop telegram-sendmail.service 2>/dev/null || true
	systemctl daemon-reload || true
fi
