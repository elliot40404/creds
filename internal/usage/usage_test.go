package usage

import (
	"math"
	"testing"
	"time"
	"uuid"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

func id(b byte) uuid.UUID {
	var u uuid.UUID
	u[15] = b
	return u
}

func TestRankHalvesEachHalfLife(t *testing.T) {
	u := Use{Score: 8, Last: t0.Unix()}
	cases := map[time.Duration]float64{0: 8, HalfLife: 4, 2 * HalfLife: 2, -time.Hour: 8}
	for age, want := range cases {
		if got := u.Rank(t0.Add(age)); math.Abs(got-want) > 1e-9 {
			t.Errorf("age %v rank %v, want %v", age, got, want)
		}
	}
}

func TestAddDecaysThenCounts(t *testing.T) {
	l := Log{}
	l.Add(id(1), t0)
	l.Add(id(1), t0)
	if got := l[id(1)].Score; got != 2 {
		t.Fatalf("score %v", got)
	}
	l.Add(id(1), t0.Add(HalfLife))
	if got := l[id(1)].Score; math.Abs(got-2) > 1e-9 {
		t.Fatalf("score after a half life %v, want 2", got)
	}
}

func TestOftenOldVersusOnceNow(t *testing.T) {
	l := Log{}
	month := t0.Add(-30 * 24 * time.Hour)
	for range 10 {
		l.Add(id(1), month)
	}
	l.Add(id(2), t0)
	r := l.Ranks(t0)
	if r[id(1)] <= r[id(2)] {
		t.Fatalf("ten uses a month ago %v should beat one use now %v", r[id(1)], r[id(2)])
	}
	l.Add(id(2), t0)
	l.Add(id(2), t0)
	r = l.Ranks(t0)
	if r[id(2)] <= r[id(1)] {
		t.Fatalf("three uses now %v should beat ten a month ago %v", r[id(2)], r[id(1)])
	}
}

func TestPruneForgetsOldEntries(t *testing.T) {
	l := Log{}
	l.Add(id(1), t0)
	l.Add(id(2), t0.Add(7*HalfLife))
	if _, ok := l[id(1)]; ok {
		t.Fatal("faded entry kept")
	}
	if _, ok := l[id(2)]; !ok {
		t.Fatal("new entry lost")
	}
}

func TestPruneCapsEntries(t *testing.T) {
	l := Log{}
	for i := range MaxEntries {
		var u uuid.UUID
		u[0], u[1] = byte(i>>8), byte(i)
		l[u] = Use{Score: float64(5 + i), Last: t0.Unix()}
	}
	l.Add(id(1), t0)
	if len(l) != MaxEntries {
		t.Fatalf("len %d", len(l))
	}
	if _, ok := l[id(1)]; !ok {
		t.Fatal("the entry just used was dropped")
	}
	if _, ok := l[uuid.UUID{}]; ok {
		t.Fatal("lowest rank kept over the cap")
	}
}
