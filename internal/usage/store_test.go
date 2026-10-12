package usage

import (
	"bytes"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
	"uuid"

	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/testutil"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	return &Store{Path: filepath.Join(t.TempDir(), "home", "usage.age"), Now: func() time.Time { return t0 }}
}

func TestRecordRoundTrip(t *testing.T) {
	s, key := newStore(t), testutil.Identity(t)
	for range 2 {
		if err := s.Record(key, id(1)); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Record(key, id(2)); err != nil {
		t.Fatal(err)
	}
	r := s.Ranks(key)
	if r[id(1)] != 2 || r[id(2)] != 1 || len(r) != 2 {
		t.Fatalf("ranks %v", r)
	}
}

func TestFileIsEncrypted(t *testing.T) {
	s, key := newStore(t), testutil.Identity(t)
	if err := s.Record(key, id(7)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !crypto.IsAgeFile(data) || bytes.Contains(data, []byte(id(7).String())) || bytes.Contains(data, []byte("score")) {
		t.Fatal("usage file is not sealed")
	}
}

func TestBadFileStartsFresh(t *testing.T) {
	key := testutil.Identity(t)
	other, err := crypto.NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	sealed := func(plain string) []byte {
		out, err := crypto.Seal(key.Recipient(), []byte(plain))
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	foreign, err := crypto.Seal(other.Recipient(), []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	record := func(score float64, last int64) []byte {
		l := Log{id(1): {Score: score, Last: last}}
		return encode(l)
	}
	cases := map[string][]byte{
		"junk":        []byte("not age at all"),
		"other vault": foreign,
		"bad magic":   sealed("XXXX0000"),
		"short":       sealed("CUS10002"),
		"trailing":    sealed(string(record(1, 1)) + "x"),
		"bad score":   sealed(string(record(-1, 1))),
		"nan score":   sealed(string(record(math.NaN(), 1))),
		"bad time":    sealed(string(record(1, -5))),
		"empty":       sealed(""),
		"sign count":  sealed("CUS1+001"),
		"too large":   bytes.Repeat([]byte("a"), maxFileSize+1),
		"too many":    sealed("CUS14096"),
		"dup id":      sealed("CUS10002" + string(record(1, 1)[headerSize:]) + string(record(1, 1)[headerSize:])),
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			s := newStore(t)
			if err := fsutil.EnsureDir(filepath.Dir(s.Path)); err != nil {
				t.Fatal(err)
			}
			if err := fsutil.WriteFileAtomic(s.Path, data); err != nil {
				t.Fatal(err)
			}
			if r := s.Ranks(key); len(r) != 0 {
				t.Fatalf("ranks %v", r)
			}
			if err := s.Record(key, id(1)); err != nil {
				t.Fatal(err)
			}
			if r := s.Ranks(key); r[id(1)] != 1 || len(r) != 1 {
				t.Fatalf("after record %v", r)
			}
		})
	}
}

func TestConcurrentRecordsKeepEveryUse(t *testing.T) {
	s, key := newStore(t), testutil.Identity(t)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if err := s.Record(key, id(1)); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if got := s.Ranks(key)[id(1)]; got != 8 {
		t.Fatalf("score %v, want 8", got)
	}
}

func BenchmarkRecord(b *testing.B) {
	s := &Store{Path: filepath.Join(b.TempDir(), "usage.age"), Now: func() time.Time { return t0 }}
	key := testutil.Identity(b)
	for i := range 500 {
		var u uuid.UUID
		u[0], u[1] = byte(i>>8), byte(i)
		if err := s.Record(key, u); err != nil {
			b.Fatal(err)
		}
	}
	for b.Loop() {
		if err := s.Record(key, id(1)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRanks(b *testing.B) {
	s := &Store{Path: filepath.Join(b.TempDir(), "usage.age"), Now: func() time.Time { return t0 }}
	key := testutil.Identity(b)
	for i := range 500 {
		var u uuid.UUID
		u[0], u[1] = byte(i>>8), byte(i)
		if err := s.Record(key, u); err != nil {
			b.Fatal(err)
		}
	}
	for b.Loop() {
		s.Ranks(key)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	l := Log{id(1): {Score: 2.5, Last: t0.Unix()}, id(2): {Score: 0.25, Last: 1}}
	got, ok := decode(crypto.Pad(encode(l)))
	if !ok || len(got) != 2 || got[id(1)] != l[id(1)] || got[id(2)] != l[id(2)] {
		t.Fatalf("got %v %v", got, ok)
	}
}
