package manifest

import (
	"encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/vaultfiles"
)

func hash(b byte) string {
	return strings.Repeat(string("0123456789abcdef"[b%16]), hashLen)
}

func goodFiles() map[string]string {
	out := map[string]string{}
	for i, p := range vaultfiles.Covered() {
		out[p] = hash(byte(i))
	}
	return out
}

func good(t *testing.T) Manifest {
	t.Helper()
	m, err := New(1, nil, goodFiles())
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func raw(t *testing.T, m Manifest) []byte {
	t.Helper()
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestRoundtrip(t *testing.T) {
	t.Parallel()
	m := good(t)
	back, err := parsePayload(raw(t, m))
	if err != nil {
		t.Fatal(err)
	}
	a, _ := m.ID()
	b, _ := back.ID()
	if a != b || a == "" {
		t.Fatalf("id %q vs %q", a, b)
	}
}

func TestIDIsStable(t *testing.T) {
	t.Parallel()
	a, err := New(1, nil, goodFiles())
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(1, nil, goodFiles())
	if err != nil {
		t.Fatal(err)
	}
	ida, _ := a.ID()
	idb, _ := b.ID()
	if ida != idb {
		t.Fatalf("%s != %s", ida, idb)
	}
}

func TestNextCountsUp(t *testing.T) {
	t.Parallel()
	first := good(t)
	second, err := Next([]Manifest{first}, goodFiles())
	if err != nil {
		t.Fatal(err)
	}
	if second.Generation != 2 || len(second.Parents) != 1 {
		t.Fatalf("gen %d parents %v", second.Generation, second.Parents)
	}
	id, _ := first.ID()
	if second.Parents[0] != id {
		t.Fatalf("parent %q want %q", second.Parents[0], id)
	}
}

func TestNextTakesTheHigherParentGeneration(t *testing.T) {
	t.Parallel()
	a, _ := New(3, nil, goodFiles())
	b, _ := New(7, nil, goodFiles())
	merged, err := Next([]Manifest{a, b}, goodFiles())
	if err != nil {
		t.Fatal(err)
	}
	if merged.Generation != 8 || len(merged.Parents) != 2 {
		t.Fatalf("gen %d parents %v", merged.Generation, merged.Parents)
	}
	if merged.Parents[0] >= merged.Parents[1] {
		t.Fatalf("parents not sorted: %v", merged.Parents)
	}
}

func TestNextRefusesOverflow(t *testing.T) {
	t.Parallel()
	top, err := New(^uint64(0), nil, goodFiles())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Next([]Manifest{top}, goodFiles()); !errors.Is(err, ErrOverflow) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		edit func(Manifest) Manifest
	}{
		{"bad format", func(m Manifest) Manifest { m.Format = 2; return m }},
		{"zero generation", func(m Manifest) Manifest { m.Generation = 0; return m }},
		{"three parents", func(m Manifest) Manifest {
			m.Parents = []string{hash(1), hash(2), hash(3)}
			return m
		}},
		{"repeated parent", func(m Manifest) Manifest { m.Parents = []string{hash(1), hash(1)}; return m }},
		{"unsorted parents", func(m Manifest) Manifest { m.Parents = []string{hash(9), hash(1)}; return m }},
		{"short hash", func(m Manifest) Manifest { m.Files[0].Hash = "abc"; return m }},
		{"upper hash", func(m Manifest) Manifest { m.Files[0].Hash = strings.ToUpper(hash(10)); return m }},
		{"missing file", func(m Manifest) Manifest { m.Files = m.Files[1:]; return m }},
		{"unknown path", func(m Manifest) Manifest { m.Files[0].Path = "evil.txt"; return m }},
		{"unsorted files", func(m Manifest) Manifest {
			m.Files[0], m.Files[1] = m.Files[1], m.Files[0]
			return m
		}},
		{"duplicate path", func(m Manifest) Manifest { m.Files[1].Path = m.Files[0].Path; return m }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := raw(t, tt.edit(good(t)))
			if _, err := parsePayload(data); !errors.Is(err, ErrBadManifest) {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestParseRejectsUnknownMember(t *testing.T) {
	t.Parallel()
	data := raw(t, good(t))
	data = append([]byte(`{"evil":1,`), data[1:]...)
	if _, err := parsePayload(data); !errors.Is(err, ErrBadManifest) {
		t.Fatalf("err = %v", err)
	}
}

func TestParseRejectsNonCanonicalBytes(t *testing.T) {
	t.Parallel()
	data := append([]byte(" "), raw(t, good(t))...)
	if _, err := parsePayload(data); !errors.Is(err, ErrBadManifest) {
		t.Fatalf("err = %v", err)
	}
}
