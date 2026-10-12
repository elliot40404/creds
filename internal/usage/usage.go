package usage

import (
	"cmp"
	"math"
	"slices"
	"time"
	"uuid"
)

const (
	HalfLife   = 14 * 24 * time.Hour
	MaxEntries = 2000
	minRank    = 0.01
)

type Use struct {
	Score float64 `json:"score"`
	Last  int64   `json:"last"`
}

type Log map[uuid.UUID]Use

func (u Use) Rank(now time.Time) float64 {
	age := max(now.Sub(time.Unix(u.Last, 0)), 0)
	return u.Score * math.Exp2(-float64(age)/float64(HalfLife))
}

func (l Log) Add(id uuid.UUID, now time.Time) {
	l[id] = Use{Score: l[id].Rank(now) + 1, Last: now.Unix()}
	l.prune(now, id)
}

func (l Log) Ranks(now time.Time) map[uuid.UUID]float64 {
	out := make(map[uuid.UUID]float64, len(l))
	for id, u := range l {
		out[id] = u.Rank(now)
	}
	return out
}

func (l Log) prune(now time.Time, keep uuid.UUID) {
	for id, u := range l {
		if u.Rank(now) < minRank {
			delete(l, id)
		}
	}
	if len(l) <= MaxEntries {
		return
	}
	ranks := l.Ranks(now)
	ranks[keep] = math.Inf(1)
	ids := make([]uuid.UUID, 0, len(l))
	for id := range l {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return cmp.Compare(ranks[b], ranks[a]) })
	for _, id := range ids[MaxEntries:] {
		delete(l, id)
	}
}
