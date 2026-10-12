package usage

import "testing"

func FuzzDecode(f *testing.F) {
	f.Add(encode(Log{id(1): {Score: 1, Last: 1}}))
	f.Add([]byte("CUS10000"))
	f.Add([]byte("CUS19999"))
	f.Fuzz(func(t *testing.T, data []byte) {
		l, ok := decode(data)
		if !ok {
			return
		}
		again, ok := decode(encode(l))
		if !ok || len(again) != len(l) {
			t.Fatalf("round trip lost entries %d %d", len(l), len(again))
		}
		for id, u := range l {
			if again[id] != u || !u.valid() {
				t.Fatalf("entry %v changed", id)
			}
		}
	})
}
