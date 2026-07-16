package main

import (
	"strings"
	"testing"
)

func TestRenderSystemdUnits(t *testing.T) {
	const exe = "/usr/local/bin/telegram-sendmail"
	out := renderSystemdUnits(exe)

	// Socket path and mode must stay world-writable (system mailer; see AGENTS.md /
	// CONSISTENTLY_IGNORED: do not tighten SocketMode).
	for _, want := range []string{
		"ListenStream=/run/telegram-sendmail/socket.sock",
		"SocketMode=0777",
		"ExecStart=" + exe + " serve",
		"StateDirectory=telegram-sendmail",
		// Align with nixos-module.nix serviceConfig so unit installs match NixOS.
		"RuntimeDirectory=telegram-sendmail",
		"RuntimeDirectoryPreserve=yes",
		"Restart=on-failure",
		"RestartSec=1",
		"EnvironmentFile=/etc/telegram-sendmail.env",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("unit output missing %q\n---\n%s\n---", want, out)
		}
	}

	if strings.Count(out, "[Unit]") != 2 {
		t.Errorf("expected socket + service [Unit] sections, got %d", strings.Count(out, "[Unit]"))
	}
}
