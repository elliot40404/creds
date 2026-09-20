package clipboard

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	helperEnv  = "CREDS_CLEAR_HELPER_DIR"
	helperFile = "child"
)

func TestMain(m *testing.M) {
	if out := os.Getenv(helperEnv); out != "" {
		os.Exit(helper(out))
	}
	os.Exit(m.Run())
}

func helper(dir string) int {
	hash, err := readHash(os.Stdin)
	if err != nil {
		return 1
	}
	data := strings.Join(append([]string{hash}, os.Args[1:]...), "\n")
	root, err := os.OpenRoot(dir)
	if err != nil {
		return 1
	}
	defer func() { _ = root.Close() }()
	if err := root.WriteFile(helperFile, []byte(data), 0o600); err != nil {
		return 1
	}
	return 0
}

func TestClearCmdKeepsValueOutOfArgsAndEnv(t *testing.T) {
	hash := Hash(secretValue)
	cmd, in, err := clearCmd("creds", ClearJob{After: 30 * time.Second, Mode: ModeNative, Hash: hash})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close() }()
	want := []string{"creds", ClearCommand, "--after", "30s", "--mode", "native"}
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("args = %q", cmd.Args)
	}
	for _, kv := range append(slices.Clone(cmd.Args), cmd.Env...) {
		if strings.Contains(kv, secretValue) || strings.Contains(kv, hash) {
			t.Fatalf("leak in %q", kv)
		}
	}
	got, err := io.ReadAll(in)
	if err != nil || string(got) != hash+"\n" {
		t.Fatalf("stdin = %q, %v", got, err)
	}
}

func TestClearCmdRejects(t *testing.T) {
	if _, _, err := clearCmd("creds", ClearJob{Mode: ModeAuto}); !errors.Is(err, ErrBadMode) {
		t.Fatalf("auto: %v", err)
	}
	if _, _, err := clearCmd("creds", ClearJob{Mode: ModeOSC52}); !errors.Is(err, ErrNoTTY) {
		t.Fatalf("osc52 without tty: %v", err)
	}
}

func TestSpawnDeliversHash(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(helperEnv, dir)
	out := filepath.Join(dir, helperFile)
	hash := Hash(secretValue)
	if err := Spawn(ClearJob{After: time.Second, Mode: ModeNative, Hash: hash}); err != nil {
		t.Fatal(err)
	}
	var data []byte
	for range 200 {
		b, err := os.ReadFile(filepath.Clean(out))
		if err == nil && len(b) > 0 {
			data = b
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	want := strings.Join([]string{hash, ClearCommand, "--after", "1s", "--mode", "native"}, "\n")
	if string(data) != want {
		t.Fatalf("child saw %q", data)
	}
}
