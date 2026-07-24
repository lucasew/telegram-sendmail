// Package version holds the product version string injected at link time.
package version

import (
	"runtime/debug"
	"strings"
)

// Set via goreleaser ldflags:
// -X github.com/lucasew/telegram-sendmail/internal/version.version={{ .Version }}
var version = "dev"

// Version returns the release version.
// Preference order:
//  1. non-empty ldflags value (goreleaser release builds)
//  2. module version from runtime/debug build info (go install @vX.Y.Z)
//  3. "dev" for plain local builds
func Version() string {
	return resolveVersion(version, moduleVersion())
}

// resolveVersion picks the effective version string. Extracted for tests.
// ldflags "dev" or empty means unset so go install module versions can surface.
func resolveVersion(ldflag, moduleVer string) string {
	if v := normalizeVersion(ldflag); v != "" && v != "dev" {
		return v
	}
	if v := normalizeVersion(moduleVer); v != "" {
		return v
	}
	return "dev"
}

// normalizeVersion trims space, drops Go's "(devel)" placeholder, and strips a
// leading "v" so versions match SPEC tags (no v prefix).
func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "(devel)" {
		return ""
	}
	return strings.TrimPrefix(v, "v")
}

func moduleVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return info.Main.Version
}

// BuildID returns a short VCS revision when available, otherwise "unknown".
func BuildID() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			rev := strings.TrimSpace(setting.Value)
			if rev == "" {
				return "unknown"
			}
			if len(rev) > 8 {
				return rev[:8]
			}
			return rev
		}
	}
	return "unknown"
}

// GetBuildID returns version combined with a short commit hash.
func GetBuildID() string {
	return Version() + "-" + BuildID()
}
