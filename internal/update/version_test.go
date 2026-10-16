package update

import (
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.3.0", "0.2.1", true},
		{"v0.2.1", "0.2.1", false},
		{"v0.2.0", "0.2.1", false},
		{"v1.0.0", "0.9.9", true},
		{"v0.10.0", "0.9.0", true},
		{"v0.2.10", "0.2.9", true},
		{"v0.3.0", "v0.3.0-rc1", true},
		{"v0.3.0-rc2", "0.3.0-rc1", true},
		{"v0.3.0-rc1", "0.3.0", false},
		{"v0.3.0+meta", "0.3.0", false},
		{"v0.3.0", "dev", false},
		{"v0.3.0", "", false},
		{"v0.3.0", "0.2", false},
		{"latest", "0.2.1", false},
		{"v0.3.0-", "0.2.1", false},
		{"v0.x.0", "0.2.1", false},
		{"v0..0", "0.2.1", false},
		{"v0.3.0.1", "0.2.1", false},
	}
	for _, tc := range cases {
		if got := Newer(tc.latest, tc.current); got != tc.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func TestDue(t *testing.T) {
	now := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		checked time.Time
		want    bool
	}{
		{"never", time.Time{}, true},
		{"just now", now, false},
		{"an hour ago", now.Add(-time.Hour), false},
		{"almost a day", now.Add(-Every + time.Second), false},
		{"a day", now.Add(-Every), true},
		{"a week", now.Add(-7 * Every), true},
		{"clock moved back", now.Add(time.Hour), true},
	}
	for _, tc := range cases {
		if got := Due(tc.checked, now); got != tc.want {
			t.Errorf("%s: Due = %v, want %v", tc.name, got, tc.want)
		}
	}
}
