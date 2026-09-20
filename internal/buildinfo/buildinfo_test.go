package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

func TestGetFallsBackToDev(t *testing.T) {
	got := Get()
	if got.Version == "" {
		t.Fatal("version is empty")
	}
	if got.Go != runtime.Version() {
		t.Fatalf("go = %q, want %q", got.Go, runtime.Version())
	}
	if got.OS != runtime.GOOS || got.Arch != runtime.GOARCH {
		t.Fatalf("platform = %s/%s, want %s/%s", got.OS, got.Arch, runtime.GOOS, runtime.GOARCH)
	}
}

func TestString(t *testing.T) {
	info := Info{Version: "v1.2.3", Commit: "0123456789abcdef", Date: "2026-01-01T00:00:00Z", Go: "go1.27.1", OS: "linux", Arch: "amd64"}
	want := "creds v1.2.3 (0123456789ab) 2026-01-01T00:00:00Z go1.27.1 linux/amd64"
	if got := info.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestStringWithoutCommit(t *testing.T) {
	info := Info{Version: "dev", Go: "go1.27.1", OS: "linux", Arch: "amd64"}
	if got := info.String(); got != "creds dev go1.27.1 linux/amd64" {
		t.Fatalf("String() = %q", got)
	}
}

func TestShortKeepsDirty(t *testing.T) {
	if got := short("0123456789abcdef-dirty"); got != "0123456789ab-dirty" {
		t.Fatalf("short() = %q", got)
	}
}

func TestOrDev(t *testing.T) {
	if got := (Info{}).orDev().Version; got != "dev" {
		t.Fatalf("orDev() = %q, want dev", got)
	}
}

func TestStringHasNoNewline(t *testing.T) {
	if strings.Contains(Get().String(), "\n") {
		t.Fatal("version string contains a newline")
	}
}
