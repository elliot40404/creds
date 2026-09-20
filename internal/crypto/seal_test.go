package crypto

import (
	"bytes"
	"errors"
	"testing"
)

func TestPadSizes(t *testing.T) {
	cases := map[int]int{0: 4096, 1: 4096, 4095: 4096, 4096: 4096, 4097: 8192, 9000: 16384}
	for in, want := range cases {
		plain := bytes.Repeat([]byte("x"), in)
		got := Pad(plain)
		if len(got) != want {
			t.Fatalf("%d: got %d want %d", in, len(got), want)
		}
		if !bytes.Equal(got[:in], plain) {
			t.Fatalf("%d: prefix changed", in)
		}
		if len(bytes.Trim(got[in:], " ")) != 0 {
			t.Fatalf("%d: padding not spaces", in)
		}
	}
}

func newSealed(t *testing.T, plain []byte) (*Identity, []byte) {
	t.Helper()
	id, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	data, err := Seal(id.Recipient(), plain)
	if err != nil {
		t.Fatal(err)
	}
	return id, data
}

func TestSealOpenRoundtrip(t *testing.T) {
	for _, n := range []int{0, 4095, 4096, 4097, 9000} {
		plain := bytes.Repeat([]byte("s"), n)
		id, data := newSealed(t, plain)
		if !IsAgeFile(data) {
			t.Fatalf("%d: missing age header", n)
		}
		if n > 0 && bytes.Contains(data, plain[:min(n, 64)]) {
			t.Fatalf("%d: plaintext visible", n)
		}
		got, err := Open(id, data)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, Pad(plain)) {
			t.Fatalf("%d: roundtrip mismatch", n)
		}
	}
}

func TestSealPaddedLengthHidden(t *testing.T) {
	_, a := newSealed(t, []byte("{}"))
	_, b := newSealed(t, bytes.Repeat([]byte("s"), 4000))
	if len(a) != len(b) {
		t.Fatalf("sizes differ: %d %d", len(a), len(b))
	}
}

func TestOpenWrongIdentity(t *testing.T) {
	_, data := newSealed(t, []byte("{}"))
	other, err := NewIdentity()
	if err != nil {
		t.Fatal(err)
	}
	_, err = Open(other, data)
	if !errors.Is(err, ErrWrongKey) || errors.Is(err, ErrWrongSecret) {
		t.Fatalf("got %v", err)
	}
}

func TestOpenCorrupted(t *testing.T) {
	id, data := newSealed(t, []byte("{}"))
	data[len(data)-10] ^= 0x01
	if _, err := Open(id, data); err == nil {
		t.Fatal("expected error")
	}
}

func TestOpenTruncated(t *testing.T) {
	id, data := newSealed(t, []byte("{}"))
	for _, n := range []int{0, 5, len(data) / 2, len(data) - 1} {
		if _, err := Open(id, data[:n]); err == nil {
			t.Fatalf("len %d: expected error", n)
		}
	}
}

func TestIsAgeFile(t *testing.T) {
	if IsAgeFile([]byte("{\"format\":1}")) {
		t.Fatal("plain json treated as age")
	}
	_, data := newWrapped(t)
	if !IsAgeFile(data) {
		t.Fatal("wrapped identity not detected")
	}
}
