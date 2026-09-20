package config

import (
	"fmt"
	"time"
)

const (
	minDuration = time.Second
	maxSession  = 24 * time.Hour
	maxClip     = 24 * time.Hour
	maxStale    = 30 * 24 * time.Hour
	maxAfter    = 5 * time.Minute
	maxEvery    = time.Hour
)

func checkLimits(c Config) error {
	for _, f := range staticFields() {
		if f.limit == nil {
			continue
		}
		d := f.limit.get(c)
		if d < minDuration {
			return fmt.Errorf("%s must be at least %s, got %s", f.Key, minDuration, d)
		}
		if d > f.limit.max {
			return fmt.Errorf("%s must be at most %s, got %s", f.Key, f.limit.max, d)
		}
	}
	return nil
}

func Warnings(c Config) []string {
	d := Default()
	var out []string
	for _, f := range staticFields() {
		if l := f.limit; l != nil && l.get(c) > l.warn {
			out = append(out, fmt.Sprintf("%s is %s, far above the %s default: %s", f.Key, l.get(c), l.get(d), l.risk))
		}
	}
	return out
}
