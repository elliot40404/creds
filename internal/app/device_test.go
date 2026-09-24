package app

import (
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/device"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/testutil"
)

func pluginKey(t *testing.T) string {
	t.Helper()
	id, rec := testutil.Plugin(t)
	return "# public key: " + rec + "\n" + id + "\n"
}

func trustedVault(t *testing.T) (*Service, *fakePrompter, *clock) {
	t.Helper()
	s, fp, c, _ := initVault(t)
	fp.passwords = []string{mainWord}
	if err := s.DeviceTrust(pluginKey(t)); err != nil {
		t.Fatal(err)
	}
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	fp.prompts, fp.warned = nil, nil
	return s, fp, c
}

func trusted(t *testing.T, s *Service) bool {
	t.Helper()
	st, err := s.DeviceStatus()
	if err != nil {
		t.Fatal(err)
	}
	return st.Trusted
}

func warnedWith(fp *fakePrompter, part string) bool {
	return slices.ContainsFunc(fp.warned, func(w string) bool { return strings.Contains(w, part) })
}

func TestDeviceTrustAsksPassword(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	fp.prompts = nil
	fp.passwords = []string{mainWord}
	if err := s.DeviceTrust(pluginKey(t)); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(fp.prompts, "Master password") || !trusted(t, s) {
		t.Fatalf("prompts %v", fp.prompts)
	}
}

func TestDeviceTrustRefusesBadKey(t *testing.T) {
	t.Parallel()
	s, _, _, _ := initVault(t)
	if err := s.DeviceTrust("AGE-SECRET-KEY-1XYZ"); !errors.Is(err, device.ErrBadKey) {
		t.Fatalf("got %v", err)
	}
}

func TestDeviceUnlockWithoutPassword(t *testing.T) {
	t.Parallel()
	s, fp, _ := trustedVault(t)
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	if len(fp.prompts) != 0 {
		t.Fatalf("prompted %v", fp.prompts)
	}
	if _, err := s.sessions().Load(); err != nil {
		t.Fatalf("session not saved: %v", err)
	}
}

func TestDeviceUnlockWithDoesNotTouch(t *testing.T) {
	s, fp, _ := trustedVault(t)
	t.Setenv(testutil.PluginModeEnv, "cancel")
	if err := s.UnlockWith(mainWord); err != nil {
		t.Fatal(err)
	}
	if len(fp.warned) != 0 {
		t.Fatalf("device step ran: %v", fp.warned)
	}
}

func TestDeviceCancelFallsBackToPassword(t *testing.T) {
	s, fp, _ := trustedVault(t)
	t.Setenv(testutil.PluginModeEnv, "cancel")
	fp.passwords = []string{mainWord}
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	if !warnedWith(fp, "device unlock skipped") || !slices.Contains(fp.prompts, "Master password") {
		t.Fatalf("warned %v prompts %v", fp.warned, fp.prompts)
	}
	if !trusted(t, s) {
		t.Fatal("cancel removed trust")
	}
}

func TestDevicePINGoesToPrompter(t *testing.T) {
	s, fp, _ := trustedVault(t)
	t.Setenv(testutil.PluginModeEnv, "pin")
	fp.passwords = []string{testutil.PluginPIN}
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(fp.prompts, []string{"credstest plugin: PIN"}) {
		t.Fatalf("prompts %v", fp.prompts)
	}
}

func TestDeviceMessageGoesToWarn(t *testing.T) {
	s, fp, _ := trustedVault(t)
	t.Setenv(testutil.PluginModeEnv, "msg")
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	if !warnedWith(fp, "credstest plugin: touch your key") {
		t.Fatalf("warned %v", fp.warned)
	}
}

func TestDeviceStaleNeedsPasswordOnce(t *testing.T) {
	t.Parallel()
	s, fp, c := trustedVault(t)
	c.t = c.t.Add(s.Config.Device.MaxAge)
	fp.passwords = []string{mainWord}
	if err := s.Unlock(); err != nil {
		t.Fatal(err)
	}
	if !warnedWith(fp, "max_age") || len(fp.prompts) != 1 {
		t.Fatalf("warned %v prompts %v", fp.warned, fp.prompts)
	}
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	fp.prompts = nil
	if err := s.Unlock(); err != nil || len(fp.prompts) != 0 {
		t.Fatalf("after password: %v prompts %v", err, fp.prompts)
	}
}

func TestDeviceVaultChangedDropsTrust(t *testing.T) {
	t.Parallel()
	a, _, _ := trustedVault(t)
	b, fb, _, _ := initVault(t)
	data, err := os.ReadFile(a.Paths.Device())
	if err != nil {
		t.Fatal(err)
	}
	if err := fsutil.WriteFileAtomic(b.Paths.Device(), data); err != nil {
		t.Fatal(err)
	}
	if err := b.Lock(); err != nil {
		t.Fatal(err)
	}
	fb.passwords = []string{mainWord}
	if err := b.Unlock(); err != nil {
		t.Fatal(err)
	}
	if !warnedWith(fb, "vault key changed") || trusted(t, b) {
		t.Fatalf("warned %v", fb.warned)
	}
}

func TestPasswdOffersUntrust(t *testing.T) {
	t.Parallel()
	for _, untrust := range []bool{true, false} {
		s, fp, _ := trustedVault(t)
		fp.passwords = []string{mainWord, nextWord, nextWord}
		fp.confirms = []bool{untrust}
		if err := s.Passwd(); err != nil {
			t.Fatal(err)
		}
		if trusted(t, s) == untrust {
			t.Fatalf("untrust %v: trust left %v", untrust, !untrust)
		}
	}
}

func TestPasswdWithoutTrustAsksNothing(t *testing.T) {
	t.Parallel()
	s, fp, _, _ := initVault(t)
	fp.passwords = []string{mainWord, nextWord, nextWord}
	if err := s.Passwd(); err != nil {
		t.Fatal(err)
	}
	if len(fp.confirms) != 0 || slices.ContainsFunc(fp.prompts, func(p string) bool { return strings.Contains(p, "Untrust") }) {
		t.Fatalf("prompts %v", fp.prompts)
	}
}

func TestDeviceUntrust(t *testing.T) {
	t.Parallel()
	s, _, _ := trustedVault(t)
	if err := s.DeviceUntrust(); err != nil || trusted(t, s) {
		t.Fatalf("untrust %v", err)
	}
	if err := s.DeviceUntrust(); !errors.Is(err, device.ErrNotTrusted) {
		t.Fatalf("second untrust %v", err)
	}
}
