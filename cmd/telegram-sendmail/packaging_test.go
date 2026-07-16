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
		"RuntimeDirectory=telegram-sendmail",
		"RuntimeDirectoryPreserve=yes",
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
	}

	for _, want := range []string{
		"ListenStream=/run/telegram-sendmail/socket.sock",
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

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
