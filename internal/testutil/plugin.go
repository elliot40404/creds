package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"filippo.io/age"
	"filippo.io/age/plugin"
)

const (
	PluginName    = "credstest"
	PluginModeEnv = "CREDS_TEST_PLUGIN"
	PluginPIN     = "1234"
	pluginPkg     = "github.com/elliot40404/creds/internal/testutil/credstest"
)

var pluginDir string

var buildPlugin = sync.OnceValue(func() error {
	name := "age-plugin-" + PluginName
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", filepath.Join(pluginDir, name), pluginPkg).CombinedOutput()
	if err != nil {
		return fmt.Errorf("build fake plugin: %w\n%s", err, out)
	}
	return nil
})

func addPluginDir(dir string) error {
	pluginDir = filepath.Join(dir, "plugin")
	if err := os.Mkdir(pluginDir, 0o700); err != nil {
		return err
	}
	return os.Setenv("PATH", pluginDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func Plugin(tb testing.TB) (identity, recipient string) {
	tb.Helper()
	if pluginDir == "" {
		tb.Fatal("testutil.Plugin needs testutil.Main in TestMain")
	}
	if err := buildPlugin(); err != nil {
		tb.Fatal(err)
	}
	k, err := age.GenerateX25519Identity()
	if err != nil {
		tb.Fatal(err)
	}
	identity = plugin.EncodeIdentity(PluginName, []byte(k.String()))
	recipient = plugin.EncodeRecipient(PluginName, []byte(k.Recipient().String()))
	return identity, recipient
}
