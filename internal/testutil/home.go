package testutil

import (
	"os"
	"path/filepath"
)

func fakeHome(dir string) error {
	keepGoDirs()
	home := filepath.Join(dir, "home")
	if err := os.Mkdir(home, 0o700); err != nil {
		return err
	}
	for _, k := range []string{"HOME", "USERPROFILE"} {
		if err := os.Setenv(k, home); err != nil {
			return err
		}
	}
	return nil
}

func keepGoDirs() {
	keepDir("GOCACHE", os.UserCacheDir, "go-build")
	keepDir("GOENV", os.UserConfigDir, "go", "env")
	keepDir("GOPATH", os.UserHomeDir, "go")
}

func keepDir(key string, base func() (string, error), elem ...string) {
	if os.Getenv(key) != "" {
		return
	}
	dir, err := base()
	if err != nil {
		return
	}
	_ = os.Setenv(key, filepath.Join(append([]string{dir}, elem...)...))
}
