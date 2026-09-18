package version

import (
	"strings"
	"testing"
)

func TestStringContainsParts(t *testing.T) {
	Version = "v1.2.3"
	Commit = "abc1234"
	BuildTime = "2026-09-17T20:00:00Z"
	defer func() {
		Version = "dev"
		Commit = "none"
		BuildTime = "unknown"
	}()

	s := String()
	for _, part := range []string{"stair-platform", "v1.2.3", "abc1234", "2026-09-17T20:00:00Z"} {
		if !strings.Contains(s, part) {
			t.Fatalf("String() = %q, want to contain %q", s, part)
		}
	}
}

func TestDefaultValues(t *testing.T) {
	if Version != "dev" || Commit != "none" || BuildTime != "unknown" {
		t.Fatalf("unexpected defaults: %q %q %q", Version, Commit, BuildTime)
	}
}
