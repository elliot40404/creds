package search

import (
	"strings"
	"sync"
	"unicode"

	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

const (
	slab16 = 16 * 1024
	slab32 = 2048
)

var _ = algo.Init("default")

var slabs = sync.Pool{New: func() any { return util.MakeSlab(slab16, slab32) }}

type term struct {
	text []rune
	fold bool
}

func newTerm(word string) term {
	fold := !strings.ContainsFunc(word, unicode.IsUpper)
	r := []rune(word)
	if fold {
		for i, c := range r {
			r[i] = unicode.ToLower(c)
		}
	}
	return term{text: algo.NormalizeRunes(r), fold: fold}
}

func terms(text string) []term {
	words := strings.Fields(text)
	out := make([]term, len(words))
	for i, w := range words {
		out[i] = newTerm(w)
	}
	return out
}

func (t term) match(c *util.Chars, withPos bool, slab *util.Slab) (int, *[]int) {
	res, pos := algo.FuzzyMatchV2(!t.fold, true, true, c, t.text, withPos, slab)
	if res.Start < 0 {
		return 0, nil
	}
	return max(res.Score, 1), pos
}
