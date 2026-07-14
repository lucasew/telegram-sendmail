package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lucasew/telegram-sendmail/internal/utils"
	"github.com/spf13/cobra"
)

const (
	// defaultExecutablePath is the fallback path if os.Executable fails.
	defaultExecutablePath = "/usr/bin/telegram-sendmail"
)

var unitCmd = &cobra.Command{
	Use:   "unit",
	Short: "Emit systemd unit files to stdout",
	Run: func(cmd *cobra.Command, args []string) {
		exe, err := os.Executable()
		if err != nil {
			exe = defaultExecutablePath
		}
		if absExe, err := filepath.Abs(exe); err == nil {
			exe = absExe
		} else {
			utils.ReportError(err, "Failed to get absolute path for executable", "exe", exe)
		}

		fmt.Printf(`; /etc/systemd/system/telegram-sendmail.socket
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
EnvironmentFile=/etc/telegram-sendmail.env
; The service handles idle timeout internally
Restart=on-failure

[Install]
WantedBy=multi-user.target
`, exe)
	},
}

func init() {
	rootCmd.AddCommand(unitCmd)
}
