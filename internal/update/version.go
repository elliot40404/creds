package update

import (
	"cmp"
	"strconv"
	"strings"
	"time"
)

const Every = 24 * time.Hour

type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

func Parse(s string) (Version, bool) {
	s = strings.TrimPrefix(s, "v")
	s, _, _ = strings.Cut(s, "+")
	core, pre, hasPre := strings.Cut(s, "-")
	if hasPre && pre == "" {
		return Version{}, false
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, false
	}
	var nums [3]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return Version{}, false
		}
		nums[i] = n
	}
	return Version{Major: nums[0], Minor: nums[1], Patch: nums[2], Pre: pre}, true
}

func (v Version) Compare(o Version) int {
	if c := cmp.Or(cmp.Compare(v.Major, o.Major), cmp.Compare(v.Minor, o.Minor), cmp.Compare(v.Patch, o.Patch)); c != 0 {
		return c
	}
	switch {
	case v.Pre == o.Pre:
		return 0
	case v.Pre == "":
		return 1
	case o.Pre == "":
		return -1
	}
	return strings.Compare(v.Pre, o.Pre)
}

func Newer(latest, current string) bool {
	l, okL := Parse(latest)
	c, okC := Parse(current)
	return okL && okC && l.Compare(c) > 0
}

func Due(checked, now time.Time) bool {
	return checked.IsZero() || now.Before(checked) || now.Sub(checked) >= Every
}
