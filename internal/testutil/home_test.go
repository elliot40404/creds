package testutil_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainHidesTheRealHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(home, filepath.Clean(os.TempDir())+string(filepath.Separator)) {
		t.Fatalf("home %q is not a temp dir", home)
	}
	for _, k := range []string{"GOCACHE", "GOPATH"} {
		if v := os.Getenv(k); v == "" || strings.HasPrefix(v, home) {
			t.Fatalf("%s = %q, want the real go dir", k, v)
		}
	}
}
