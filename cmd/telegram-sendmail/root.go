package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lucasew/telegram-sendmail/internal/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "telegram-sendmail",
	Short: "sendmail drop-in replacement that sends to Telegram",
	Long: `A sendmail replacement that forwards emails to a Telegram chat.
It uses systemd socket activation and file-based queuing for reliability.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		utils.ReportError(err, "Execution failed")
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	pFlags := rootCmd.PersistentFlags()

	// Define flags
	pFlags.StringP("state-dir", "d", "", "Where to store queue data (default: $STATE_DIRECTORY/telegram_sendmail_state or ./telegram_sendmail_state)")
	pFlags.StringP("telegram-token", "t", "", "Telegram Bot Token")
	pFlags.StringP("telegram-chat", "c", "", "Telegram Chat ID")
	pFlags.StringP("hostname", "n", "", "Hostname to identify the sender")
	pFlags.StringP("subject", "s", "Message", "Default subject")
	pFlags.Int("max-payload-size", 20*1024*1024, "Maximum allowed payload size in bytes")
	pFlags.Float64("socket-timeout", 10.0, "Socket timeout for requests (seconds)")
	pFlags.String("sentry-dsn", "", "Sentry DSN")

	// Bind flags to viper
	viper.BindPFlag("state_dir", pFlags.Lookup("state-dir"))
	viper.BindPFlag("telegram_token", pFlags.Lookup("telegram-token"))
	viper.BindPFlag("telegram_chat", pFlags.Lookup("telegram-chat"))
	viper.BindPFlag("hostname", pFlags.Lookup("hostname"))
	viper.BindPFlag("default_subject", pFlags.Lookup("subject"))
	viper.BindPFlag("max_payload_size", pFlags.Lookup("max-payload-size"))
	viper.BindPFlag("socket_timeout", pFlags.Lookup("socket-timeout"))
	viper.BindPFlag("sentry_dsn", pFlags.Lookup("sentry-dsn"))
}

func initConfig() {
	// Map environment variables
	// MAIL_TELEGRAM_TOKEN -> telegram_token
	// MAIL_TELEGRAM_CHAT -> telegram_chat
	// STATE_DIRECTORY -> state_dir (partial handling in code if needed)
	// HOSTNAME -> hostname
	// MAIL_SENTRY_DSN -> sentry_dsn

	viper.SetEnvPrefix("MAIL") // This would look for MAIL_TELEGRAM_TOKEN if SetEnvKeyReplacer is generic, but let's be specific

	// AutomaticEnv doesn't handle specific mapping easily without prefixes, so we can manual bind or use a consistent prefix.
	// The python script uses:
	// MAIL_TELEGRAM_TOKEN
	// MAIL_TELEGRAM_CHAT
	// STATE_DIRECTORY
	// HOSTNAME

	viper.BindEnv("telegram_token", "MAIL_TELEGRAM_TOKEN")
	viper.BindEnv("telegram_chat", "MAIL_TELEGRAM_CHAT")
	viper.BindEnv("state_dir", "STATE_DIRECTORY")
	viper.BindEnv("hostname", "HOSTNAME")
	viper.BindEnv("sentry_dsn", "MAIL_SENTRY_DSN")

	// Set defaults that depend on file reads or other envs
	viper.SetDefault("hostname", getDefaultHostname())
	viper.SetDefault("state_dir", getDefaultStateDir())

	// Initialize Sentry if DSN is provided
	if dsn := viper.GetString("sentry_dsn"); dsn != "" {
		if err := utils.InitSentry(dsn); err != nil {
			utils.ReportError(err, "Failed to initialize Sentry")
		}
	}
}

func getDefaultHostname() string {
	if h := os.Getenv("HOSTNAME"); h != "" {
		return h
	}
	content, err := os.ReadFile("/etc/hostname")
	if err == nil {
		return strings.TrimSpace(string(content))
	}
	return "unknown"
}

func getDefaultStateDir() string {
	if s := os.Getenv("STATE_DIRECTORY"); s != "" {
		return filepath.Join(s, "telegram_sendmail_state")
	}
	return "telegram_sendmail_state"
}
