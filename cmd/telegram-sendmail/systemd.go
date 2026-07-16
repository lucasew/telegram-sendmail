package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lucasew/telegram-sendmail/internal/utils"
	"github.com/spf13/cobra"
)

var unitCmd = &cobra.Command{
	Use:   "unit",
	Short: "Emit systemd unit files to stdout",
	Run: func(cmd *cobra.Command, args []string) {
		exe, err := os.Executable()
		if err != nil {
			exe = "/usr/bin/telegram-sendmail"
		}
		if absExe, err := filepath.Abs(exe); err == nil {
			exe = absExe
		} else {
			utils.ReportError(err, "Failed to get absolute path for executable", "exe", exe)
		}

		fmt.Print(renderSystemdUnits(exe))
	},
}

// renderSystemdUnits returns socket + service unit text for the given binary path.
// Keys mirror nixos-module.nix so non-NixOS installs get the same runtime dir,
// restart backoff, and StateDirectory layout as the NixOS module.
func renderSystemdUnits(exe string) string {
	return fmt.Sprintf(`; /etc/systemd/system/telegram-sendmail.socket
[Unit]
Description=Telegram Sendmail Socket

[Socket]
ListenStream=/run/telegram-sendmail/socket.sock
SocketMode=0777

[Install]
WantedBy=sockets.target

; ---------------------------------------------------------
; /etc/systemd/system/telegram-sendmail.service
[Unit]
Description=Telegram Sendmail Service
Requires=telegram-sendmail.socket
After=network.target

[Service]
ExecStart=%s serve
User=telegram_sendmail
Group=telegram_sendmail
StateDirectory=telegram-sendmail
RuntimeDirectory=telegram-sendmail
RuntimeDirectoryPreserve=yes
EnvironmentFile=/etc/telegram-sendmail.env
; The service handles idle timeout internally
Restart=on-failure
RestartSec=1

[Install]
WantedBy=multi-user.target
`, exe)
}

func init() {
	rootCmd.AddCommand(unitCmd)
}
