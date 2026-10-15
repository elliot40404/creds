package textdiff

import (
	"slices"
	"strings"
)

type Op byte

const (
	Same Op = ' '
	Del  Op = '-'
	Add  Op = '+'
	Gap  Op = '~'
)

const maxCells = 1 << 20

type Line struct {
	Op   Op
	Text string
}

func Lines(a, b string) []Line {
	x, y := split(a), split(b)
	pre := commonPrefix(x, y)
	head := x[:pre]
	x, y = x[pre:], y[pre:]
	suf := commonSuffix(x, y)
	out := make([]Line, 0, pre+len(x)+len(y))
	out = appendOp(out, Same, head)
	out = append(out, middle(x[:len(x)-suf], y[:len(y)-suf])...)
	return appendOp(out, Same, x[len(x)-suf:])
}

func Changed(lines []Line) bool {
	for _, l := range lines {
		if l.Op != Same {
			return true
		}
	}
	return false
}

func split(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}

func commonPrefix(x, y []string) int {
	n := 0
	for n < len(x) && n < len(y) && x[n] == y[n] {
		n++
	}
	return n
}

func commonSuffix(x, y []string) int {
	n := 0
	for n < len(x) && n < len(y) && x[len(x)-1-n] == y[len(y)-1-n] {
		n++
	}
	return n
}

func appendOp(out []Line, op Op, lines []string) []Line {
	for _, l := range lines {
		out = append(out, Line{Op: op, Text: l})
	}
	return out
}

func middle(x, y []string) []Line {
	if len(x) == 0 || len(y) == 0 || len(x)*len(y) > maxCells {
		return appendOp(appendOp(nil, Del, x), Add, y)
	}
	return walk(x, y, lcs(x, y))
}

func lcs(x, y []string) []int32 {
	w := len(y) + 1
	t := make([]int32, (len(x)+1)*w)
	for i, a := range slices.Backward(x) {
		for j, b := range slices.Backward(y) {
			if a == b {
				t[i*w+j] = t[(i+1)*w+j+1] + 1
			} else {
				t[i*w+j] = max(t[(i+1)*w+j], t[i*w+j+1])
			}
		}
	}
	return t
}

func walk(x, y []string, t []int32) []Line {
	w := len(y) + 1
	out := make([]Line, 0, len(x)+len(y))
	i, j := 0, 0
	for i < len(x) && j < len(y) {
		switch {
		case x[i] == y[j]:
			out = append(out, Line{Op: Same, Text: x[i]})
			i, j = i+1, j+1
		case t[(i+1)*w+j] >= t[i*w+j+1]:
			out = append(out, Line{Op: Del, Text: x[i]})
			i++
		default:
			out = append(out, Line{Op: Add, Text: y[j]})
			j++
		}
	}
	return appendOp(appendOp(out, Del, x[i:]), Add, y[j:])
}

func Context(lines []Line, n int) []Line {
	keep := make([]bool, len(lines))
	for i, l := range lines {
		if l.Op == Same {
			continue
		}
		for k := max(0, i-n); k <= min(len(lines)-1, i+n); k++ {
			keep[k] = true
		}
	}
	var out []Line
	for i, l := range lines {
		switch {
		case keep[i]:
			out = append(out, l)
		case len(out) == 0 || out[len(out)-1].Op != Gap:
			out = append(out, Line{Op: Gap})
		}
	}
	return out
}
