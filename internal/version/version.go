// Package version holds the product version string injected at link time.
package version

import (
	"runtime/debug"
	"strings"
)

// Set via goreleaser ldflags:
// -X github.com/lucasew/telegram-sendmail/internal/version.version={{ .Version }}
var version = "dev"

// Version returns the release version, or "dev" when unset.
func Version() string {
	v := strings.TrimSpace(version)
	if v == "" {
		return "dev"
	}
	return v
}

// BuildID returns a short VCS revision when available.
func BuildID() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			if len(setting.Value) > 8 {
				return setting.Value[:8]
			}
			return setting.Value
		}
	}
	return "dev"
}

// GetBuildID returns version combined with a short commit hash.
func GetBuildID() string {
	return Version() + "-" + BuildID()
}
