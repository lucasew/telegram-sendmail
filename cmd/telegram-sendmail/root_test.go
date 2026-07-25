package main

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func TestFirstNonEmptyHostname(t *testing.T) {
	tests := []struct {
		name       string
		candidates []string
		want       string
	}{
		{
			name:       "prefers first non-empty",
			candidates: []string{"mailhost", "kernel"},
			want:       "mailhost",
		},
		{
			name:       "skips blank etc hostname for kernel",
			candidates: []string{"  \n", "kernel-host"},
			want:       "kernel-host",
		},
		{
			name:       "skips empty string for kernel",
			candidates: []string{"", "kernel-host"},
			want:       "kernel-host",
		},
		{
			name:       "trims whitespace around preferred",
			candidates: []string{"  mailhost\n", "other"},
			want:       "mailhost",
		},
		{
			name:       "all blank yields unknown",
			candidates: []string{"", "  ", "\n"},
			want:       "unknown",
		},
		{
			name:       "no candidates yields unknown",
			candidates: nil,
			want:       "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstNonEmptyHostname(tt.candidates...)
			if got != tt.want {
				t.Fatalf("firstNonEmptyHostname(%q)=%q want %q", tt.candidates, got, tt.want)
			}
		})
	}
}

// TestGetDefaultHostnameNeverEmpty is a smoke check against the live
// /etc/hostname + os.Hostname() sources on the runner.
func TestGetDefaultHostnameNeverEmpty(t *testing.T) {
	if got := getDefaultHostname(); got == "" {
		t.Fatal("getDefaultHostname() must never return empty string")
	}
}

// TestGetDefaultStateDirIsRelativeOnly documents that the SetDefault helper must
// not nest under STATE_DIRECTORY: BindEnv already maps that env onto state_dir
// as the queue root (systemd StateDirectory).
func TestGetDefaultStateDirIsRelativeOnly(t *testing.T) {
	t.Setenv("STATE_DIRECTORY", "/var/lib/telegram-sendmail")
	if got := getDefaultStateDir(); got != "telegram_sendmail_state" {
		t.Fatalf("getDefaultStateDir()=%q want %q (must ignore STATE_DIRECTORY)", got, "telegram_sendmail_state")
	}
}

// TestStateDirBindEnvUsesStateDirectoryAsQueueRoot locks the live precedence:
// when STATE_DIRECTORY is set and --state-dir is not changed, viper returns the
// env value as-is (not $STATE_DIRECTORY/telegram_sendmail_state).
func TestStateDirBindEnvUsesStateDirectoryAsQueueRoot(t *testing.T) {
	const envPath = "/var/lib/telegram-sendmail"
	t.Setenv("STATE_DIRECTORY", envPath)

	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	fs.String("state-dir", "", "queue directory")
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}

	v := viper.New()
	if err := v.BindPFlag("state_dir", fs.Lookup("state-dir")); err != nil {
		t.Fatalf("BindPFlag: %v", err)
	}
	if err := v.BindEnv("state_dir", "STATE_DIRECTORY"); err != nil {
		t.Fatalf("BindEnv: %v", err)
	}
	v.SetDefault("state_dir", getDefaultStateDir())

	if got := v.GetString("state_dir"); got != envPath {
		t.Fatalf("state_dir=%q want %q (BindEnv must win over nested default)", got, envPath)
	}
	if fs.Lookup("state-dir").Changed {
		t.Fatal("expected --state-dir unchanged so env can apply")
	}
}
