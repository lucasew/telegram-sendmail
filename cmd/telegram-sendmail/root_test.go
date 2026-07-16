package main

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

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
