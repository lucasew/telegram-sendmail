package version

import (
	"strings"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name      string
		ldflag    string
		moduleVer string
		want      string
	}{
		{
			name:      "ldflags wins over module",
			ldflag:    "1.2.3",
			moduleVer: "v9.9.9",
			want:      "1.2.3",
		},
		{
			name:      "ldflags trimmed",
			ldflag:    "  0.1.0  ",
			moduleVer: "",
			want:      "0.1.0",
		},
		{
			name:      "default dev falls back to module semver",
			ldflag:    "dev",
			moduleVer: "v0.4.1",
			want:      "0.4.1",
		},
		{
			name:      "empty ldflag falls back to module without v prefix",
			ldflag:    "",
			moduleVer: "0.4.1",
			want:      "0.4.1",
		},
		{
			name:      "module devel is ignored",
			ldflag:    "dev",
			moduleVer: "(devel)",
			want:      "dev",
		},
		{
			name:      "module empty yields dev",
			ldflag:    "dev",
			moduleVer: "",
			want:      "dev",
		},
		{
			name:      "whitespace module ignored",
			ldflag:    "dev",
			moduleVer: "   ",
			want:      "dev",
		},
		{
			name:      "ldflags empty string and no module",
			ldflag:    "   ",
			moduleVer: "(devel)",
			want:      "dev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveVersion(tt.ldflag, tt.moduleVer)
			if got != tt.want {
				t.Fatalf("resolveVersion(%q, %q)=%q want %q", tt.ldflag, tt.moduleVer, got, tt.want)
			}
		})
	}
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"(devel)", ""},
		{"v1.2.3", "1.2.3"},
		{"1.2.3", "1.2.3"},
		{" v0.1.0 ", "0.1.0"},
	}
	for _, tt := range tests {
		if got := normalizeVersion(tt.in); got != tt.want {
			t.Errorf("normalizeVersion(%q)=%q want %q", tt.in, got, tt.want)
		}
	}
}

func TestBuildIDNonEmpty(t *testing.T) {
	// BuildID must always return a non-empty sentinel or short hash so
	// GetBuildID never ends with a trailing dash.
	id := BuildID()
	if id == "" {
		t.Fatal("BuildID returned empty string")
	}
}

func TestGetBuildIDFormat(t *testing.T) {
	got := GetBuildID()
	if got == "" || strings.HasPrefix(got, "-") || strings.HasSuffix(got, "-") {
		t.Fatalf("GetBuildID() malformed: %q", got)
	}
	if !strings.Contains(got, "-") {
		t.Fatalf("GetBuildID() expected version-buildid, got %q", got)
	}
}
