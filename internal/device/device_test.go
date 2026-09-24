package device

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"filippo.io/age/plugin"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/testutil"
)

func TestMain(m *testing.M) { testutil.Main(m) }

var testStart = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func newStore(t *testing.T) (*Store, *clock) {
	t.Helper()
	c := &clock{t: testStart}
	return &Store{Path: filepath.Join(t.TempDir(), "device.json"), MaxAge: 72 * time.Hour, Now: c.now}, c
}

func trusted(t *testing.T) (*Store, *clock, *crypto.Identity) {
	t.Helper()
	s, c := newStore(t)
	id := testutil.Identity(t)
	pid, rec := testutil.Plugin(t)
	if err := s.Trust(id, pid, rec, &plugin.ClientUI{}); err != nil {
		t.Fatal(err)
	}
	return s, c, id
}

func vaultOf(id *crypto.Identity) string { return id.Recipient().String() }

func assertGone(t *testing.T, s *Store) {
	t.Helper()
	if _, err := os.Lstat(s.Path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("device file kept: %v", err)
	}
}

func TestTrustOpenRoundtrip(t *testing.T) {
	s, _, id := trusted(t)
	got, err := s.Open(vaultOf(id), &plugin.ClientUI{})
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != id.String() {
		t.Fatal("identity mismatch")
	}
}

func TestTrustWithIdentityAsRecipient(t *testing.T) {
	s, _ := newStore(t)
	pid, _ := testutil.Plugin(t)
	err := s.Trust(testutil.Identity(t), pid, "", &plugin.ClientUI{})
	if err == nil {
		t.Fatal("fake plugin has no identity as recipient, want error")
	}
	assertGone(t, s)
}

func TestTrustRefusesBrokenPair(t *testing.T) {
	s, _ := newStore(t)
	pid, _ := testutil.Plugin(t)
	_, rec := testutil.Plugin(t)
	if err := s.Trust(testutil.Identity(t), pid, rec, &plugin.ClientUI{}); !errors.Is(err, ErrMismatch) {
		t.Fatalf("got %v", err)
	}
	assertGone(t, s)
}

func TestTrustCanceledSavesNothing(t *testing.T) {
	t.Setenv(testutil.PluginModeEnv, "cancel")
	s, _ := newStore(t)
	pid, rec := testutil.Plugin(t)
	if err := s.Trust(testutil.Identity(t), pid, rec, &plugin.ClientUI{}); err == nil {
		t.Fatal("want error")
	}
	assertGone(t, s)
}

func TestOpenStaleAfterMaxAge(t *testing.T) {
	s, c, id := trusted(t)
	c.t = c.t.Add(s.MaxAge)
	if _, err := s.Open(vaultOf(id), &plugin.ClientUI{}); !errors.Is(err, ErrStale) {
		t.Fatalf("got %v", err)
	}
	if err := s.Confirm(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(vaultOf(id), &plugin.ClientUI{}); err != nil {
		t.Fatalf("after confirm: %v", err)
	}
}

func TestOpenMaxAgeZeroNeverStale(t *testing.T) {
	s, c, id := trusted(t)
	s.MaxAge = 0
	c.t = c.t.Add(10 * 365 * 24 * time.Hour)
	if _, err := s.Open(vaultOf(id), &plugin.ClientUI{}); err != nil {
		t.Fatal(err)
	}
}

func TestOpenVaultChangedDeletes(t *testing.T) {
	s, _, _ := trusted(t)
	if _, err := s.Open(vaultOf(testutil.Identity(t)), &plugin.ClientUI{}); !errors.Is(err, ErrVaultChanged) {
		t.Fatalf("got %v", err)
	}
	assertGone(t, s)
}

func TestOpenNotTrusted(t *testing.T) {
	s, _ := newStore(t)
	if _, err := s.Open("age1x", &plugin.ClientUI{}); !errors.Is(err, ErrNotTrusted) {
		t.Fatalf("got %v", err)
	}
}

func TestOpenPluginMissing(t *testing.T) {
	s, _, id := trusted(t)
	r, err := s.read()
	if err != nil {
		t.Fatal(err)
	}
	r.Plugin = "credsmissing"
	r.Identity = plugin.EncodeIdentity(r.Plugin, []byte("x"))
	if err := fsutil.WriteJSON(s.Path, r); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Open(vaultOf(id), &plugin.ClientUI{}); !errors.Is(err, ErrNoPlugin) {
		t.Fatalf("got %v", err)
	}
}

func TestOpenCanceledKeepsTrust(t *testing.T) {
	s, _, id := trusted(t)
	t.Setenv(testutil.PluginModeEnv, "cancel")
	if _, err := s.Open(vaultOf(id), &plugin.ClientUI{}); err == nil {
		t.Fatal("want error")
	}
	if st, err := s.Status(); err != nil || !st.Trusted {
		t.Fatalf("status %+v %v", st, err)
	}
}

func TestReadStrict(t *testing.T) {
	cases := map[string]string{
		"unknown": `{"plugin":"credstest","extra":1}`,
		"junk":    `not json`,
		"empty":   `{}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			s, _ := newStore(t)
			if err := fsutil.WriteFileAtomic(s.Path, []byte(body)); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Open("age1x", &plugin.ClientUI{}); !errors.Is(err, ErrCorrupt) {
				t.Fatalf("got %v", err)
			}
			assertGone(t, s)
		})
	}
}

func TestStatus(t *testing.T) {
	s, _ := newStore(t)
	if st, err := s.Status(); err != nil || st.Trusted {
		t.Fatalf("empty status %+v %v", st, err)
	}
	s, c, _ := trusted(t)
	c.t = c.t.Add(time.Hour)
	st, err := s.Status()
	if err != nil {
		t.Fatal(err)
	}
	want := Status{Trusted: true, Plugin: testutil.PluginName, Since: testStart, Confirmed: testStart, Expires: testStart.Add(s.MaxAge)}
	if st != want {
		t.Fatalf("got %+v want %+v", st, want)
	}
}

func TestUntrust(t *testing.T) {
	s, _, _ := trusted(t)
	if err := s.Untrust(); err != nil {
		t.Fatal(err)
	}
	assertGone(t, s)
	if err := s.Untrust(); !errors.Is(err, ErrNotTrusted) {
		t.Fatalf("second untrust: %v", err)
	}
}

func TestConfirmWithoutTrust(t *testing.T) {
	s, _ := newStore(t)
	if err := s.Confirm(); err != nil {
		t.Fatal(err)
	}
	assertGone(t, s)
}
