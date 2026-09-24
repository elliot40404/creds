package cli

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/testutil"
)

func keyFile(t *testing.T) string {
	t.Helper()
	id, rec := testutil.Plugin(t)
	path := filepath.Join(t.TempDir(), "identity.txt")
	if err := os.WriteFile(path, []byte("# public key: "+rec+"\n"+id+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func trustedHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.init()
	r := h.ok(&fake{passwords: []string{mainWord}}, "device", "trust", "--identity", keyFile(t))
	if !strings.Contains(r.err, "this device is trusted") {
		t.Fatalf("stderr %q", r.err)
	}
	h.ok(&fake{}, "lock")
	return h
}

func TestDeviceTrustThenUnlockWithoutPassword(t *testing.T) {
	t.Parallel()
	h := trustedHarness(t)
	f := &fake{}
	h.ok(f, "unlock")
	if len(f.prompts) != 0 {
		t.Fatalf("prompted %v", f.prompts)
	}
}

func TestDeviceStatus(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	if r := h.ok(&fake{}, "device", "status"); r.out != "not trusted\n" {
		t.Fatalf("out %q", r.out)
	}
	h = trustedHarness(t)
	if r := h.ok(&fake{}, "device", "status"); !strings.Contains(r.out, "trusted with the credstest plugin") {
		t.Fatalf("out %q", r.out)
	}
	r := h.ok(&fake{}, "device", "status", "--json")
	var got jsonDevice
	if err := json.Unmarshal([]byte(r.out), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Trusted || got.Plugin != testutil.PluginName || got.Stale || got.Expires.IsZero() {
		t.Fatalf("json %+v", got)
	}
}

func TestDeviceUntrust(t *testing.T) {
	t.Parallel()
	h := trustedHarness(t)
	h.ok(&fake{}, "device", "untrust")
	h.ok(&fake{passwords: []string{mainWord}}, "unlock")
	r := h.fail(&fake{}, "device", "untrust")
	if !strings.Contains(r.err, "not trusted") {
		t.Fatalf("stderr %q", r.err)
	}
}

func TestDeviceTrustNeedsOneSource(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	for _, args := range [][]string{{}, {"--touchid", "--identity", "x"}} {
		r := h.fail(&fake{}, append([]string{"device", "trust"}, args...)...)
		if !strings.Contains(r.err, "exactly one of --touchid or --identity") {
			t.Fatalf("%v: %q", args, r.err)
		}
	}
}

func TestDeviceTrustRefusesNativeKey(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	path := filepath.Join(t.TempDir(), "key.txt")
	if err := os.WriteFile(path, []byte("AGE-SECRET-KEY-1XYZ\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := h.fail(&fake{}, "device", "trust", "--identity", path)
	if !strings.Contains(r.err, "not an age plugin identity") || r.code != 2 {
		t.Fatalf("code %d stderr %q", r.code, r.err)
	}
}

func TestDeviceTrustTouchIDOffMac(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "darwin" {
		t.Skip("darwin runs the real age-plugin-se")
	}
	h := newHarness(t)
	h.init()
	r := h.fail(&fake{}, "device", "trust", "--touchid")
	if !strings.Contains(r.err, "needs macOS") {
		t.Fatalf("stderr %q", r.err)
	}
}

func TestDeviceHelpAndCompletion(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	r := h.ok(&fake{}, "device", "trust", "--help")
	for _, want := range []string{"--touchid", "--identity", "creds device trust --touchid"} {
		if !strings.Contains(r.out, want) {
			t.Fatalf("help missing %q: %q", want, r.out)
		}
	}
	out := complete(t, h, "device", "")
	for _, want := range []string{"trust", "untrust", "status"} {
		if !strings.Contains(out, want+"\t") {
			t.Fatalf("completion missing %q: %q", want, out)
		}
	}
}

func TestInteractiveDeviceTrust(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	h.init()
	path := keyFile(t)
	f := &fake{picks: []string{deviceIdentity}, inputs: []string{path}, passwords: []string{mainWord}}
	r := h.ok(f, "device", "trust", "-i")
	if argv := printedCommand(t, r.err); strings.Join(argv, " ") != "device trust --identity "+path {
		t.Fatalf("command %q", argv)
	}
	if r = h.ok(&fake{}, "device", "status"); !strings.Contains(r.out, "trusted with") {
		t.Fatalf("out %q", r.out)
	}
}
