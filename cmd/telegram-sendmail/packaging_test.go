package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// cmd/telegram-sendmail -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func TestPackagedSystemdUnits(t *testing.T) {
	root := repoRoot(t)
	service := readRepoFile(t, filepath.Join(root, "packaging/systemd/telegram-sendmail.service"))
	socket := readRepoFile(t, filepath.Join(root, "packaging/systemd/telegram-sendmail.socket"))

	for _, want := range []string{
		"ExecStart=/usr/bin/telegram-sendmail serve",
		"DynamicUser=yes",
		"StateDirectory=telegram-sendmail",
		"Restart=on-failure",
		"RestartSec=1",
		"EnvironmentFile=/etc/telegram-sendmail.env",
		"Requires=telegram-sendmail.socket",
		"After=network.target telegram-sendmail.socket",
	} {
		if !strings.Contains(service, want) {
			t.Errorf("service missing %q", want)
		}
	}
	for _, line := range strings.Split(service, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "User=") || strings.HasPrefix(line, "Group=") {
			t.Errorf("service must use DynamicUser only, not fixed identity: %q", line)
		}
		// RuntimeDirectory + DynamicUser privatizes /run/telegram-sendmail and
		// breaks world-readable socket clients (Permission denied on path).
		if strings.HasPrefix(line, "RuntimeDirectory=") {
			t.Errorf("service must not set RuntimeDirectory (socket path must stay public): %q", line)
		}
	}

	for _, want := range []string{
		"ListenStream=/run/telegram-sendmail/socket.sock",
		"DirectoryMode=0755",
		"SocketMode=0777",
	} {
		if !strings.Contains(socket, want) {
			t.Errorf("socket missing %q", want)
		}
	}
}

func TestSendmailShim(t *testing.T) {
	root := repoRoot(t)
	shim := readRepoFile(t, filepath.Join(root, "packaging/sendmail"))
	if !strings.Contains(shim, "exec /usr/bin/telegram-sendmail sendmail") {
		t.Fatalf("shim does not exec sendmail subcommand:\n%s", shim)
	}
}

func TestNewaliasesNoop(t *testing.T) {
	root := repoRoot(t)
	body := readRepoFile(t, filepath.Join(root, "packaging/newaliases"))
	if !strings.HasPrefix(strings.TrimSpace(body), "#!/bin/sh") {
		t.Fatalf("newaliases missing shebang:\n%s", body)
	}
	if !strings.Contains(body, "exit 0") {
		t.Fatalf("newaliases must be a no-op (exit 0):\n%s", body)
	}
	if strings.Contains(body, "exec ") {
		t.Fatalf("newaliases must not exec another binary:\n%s", body)
	}
}

func TestPostinstallScript(t *testing.T) {
	root := repoRoot(t)
	body := readRepoFile(t, filepath.Join(root, "packaging/scripts/postinstall.sh"))
	for _, want := range []string{
		"/etc/telegram-sendmail.env",
		"telegram-sendmail.env.example",
		"chmod 0600",
		"daemon-reload",
		"enable telegram-sendmail.socket",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("postinstall missing %q", want)
		}
	}
	// Must not start the socket on install (placeholders in seeded env).
	if strings.Contains(body, "enable --now") || strings.Contains(body, "start telegram-sendmail") {
		t.Errorf("postinstall must enable socket without starting it:\n%s", body)
	}
	if !strings.Contains(body, `[ ! -e "$ENVFILE" ]`) && !strings.Contains(body, "[ ! -e \"$ENVFILE\" ]") && !strings.Contains(body, `! -e "$ENVFILE"`) {
		// Accept either quoting style as long as we only seed when missing.
		if !strings.Contains(body, "! -e ") {
			t.Errorf("postinstall must seed env only when missing:\n%s", body)
		}
	}
}

func TestPreremoveScript(t *testing.T) {
	root := repoRoot(t)
	body := readRepoFile(t, filepath.Join(root, "packaging/scripts/preremove.sh"))
	for _, want := range []string{
		"disable --now telegram-sendmail.socket",
		"daemon-reload",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("preremove missing %q", want)
		}
	}
	// Conservative: never delete secrets or state from package scripts.
	for _, bad := range []string{
		"/etc/telegram-sendmail.env",
		"rm ",
		"StateDirectory",
	} {
		if bad == "/etc/telegram-sendmail.env" {
			// Mentioning the path in a comment is fine; deleting is not.
			if strings.Contains(body, "rm ") && strings.Contains(body, "telegram-sendmail.env") {
				t.Errorf("preremove must not remove env file:\n%s", body)
			}
			continue
		}
		if bad == "rm " && strings.Contains(body, "rm ") {
			t.Errorf("preremove must not rm files:\n%s", body)
		}
	}
}

func TestGoreleaserNFPM(t *testing.T) {
	root := repoRoot(t)
	body := readRepoFile(t, filepath.Join(root, ".goreleaser.yaml"))

	for _, want := range []string{
		// GoReleaser uses "dependencies", not nFPM's standalone "depends".
		"dependencies:",
		"systemd",
		"packaging/newaliases",
		"/usr/sbin/newaliases",
		"preremove: packaging/scripts/preremove.sh",
		"postinstall: packaging/scripts/postinstall.sh",
		"mail-transport-agent",
		"smtp-forwarder",
		"smtp-server",
		"smtpdaemon",
		"MTA",
		"/usr/sbin/sendmail",
	} {
		if want == "dependencies:" && strings.Contains(body, "\ndepends:") {
			t.Errorf("goreleaser must use dependencies: (not depends:); nFPM key differs")
		}
		if !strings.Contains(body, want) {
			t.Errorf("goreleaser missing %q", want)
		}
	}
}

// TestNixOSModule locks the in-tree NixOS module to the same unit contract as
// packaging/systemd and SPEC.md (DynamicUser, public socket path, sendmail
// subcommand wrapper). Prevents regressions like RuntimeDirectory privatizing
// /run/telegram-sendmail under DynamicUser.
func TestNixOSModule(t *testing.T) {
	root := repoRoot(t)
	body := readRepoFile(t, filepath.Join(root, "nixos-module.nix"))

	for _, want := range []string{
		`listenStreams = [ socketPath ]`,
		`DirectoryMode = "0755"`,
		`SocketMode = "0777"`,
		`DynamicUser = true`,
		`StateDirectory = serviceName`,
		`Restart = "on-failure"`,
		`RestartSec = 1`,
		`requires = [ "telegram-sendmail.socket" ]`,
		`after = [ "network.target" "telegram-sendmail.socket" ]`,
		`telegram-sendmail sendmail`,
		`/run/telegram-sendmail/socket.sock`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("nixos-module missing %q", want)
		}
	}

	// RuntimeDirectory + DynamicUser privatizes the socket parent directory.
	// Match assignments only; comments may mention the pitfall by name.
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "#") {
			continue
		}
		if strings.Contains(trim, "RuntimeDirectory") {
			t.Errorf("nixos-module must not set RuntimeDirectory (socket path must stay public): %q", trim)
		}
	}

	// Module options must not advertise CLI flags the binary does not implement.
	// Only scan non-comment lines (comments may name the banned flags).
	var nonComment strings.Builder
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "#") {
			continue
		}
		nonComment.WriteString(trim)
		nonComment.WriteByte('\n')
	}
	code := nonComment.String()
	for _, bad := range []string{"--verbose", "--debug"} {
		if strings.Contains(code, bad) {
			t.Errorf("nixos-module must not example non-existent flag %q", bad)
		}
	}
	// Wording is the Go binary, not the retired Python script.
	if strings.Contains(strings.ToLower(code), "pass to the script") {
		t.Errorf("nixos-module extraArgs description still refers to a script")
	}
}

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
