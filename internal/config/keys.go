package config

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/elliot40404/creds/internal/render"
)

const FormatPrefix = "render.formats."

type Field struct {
	Key     string
	Doc     string
	Choices []string
	get     func(*Config) string
	set     func(*Config, string) error
	unset   func(*Config)
	parse   func(string) (int64, bool)
	limit   *limit
}

type limit struct {
	get  func(Config) time.Duration
	max  time.Duration
	warn time.Duration
	risk string
	zero string
}

func choices[T any](xs []T, str func(T) string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = str(x)
	}
	return out
}

func parseDuration(v string) (int64, bool) {
	d, err := time.ParseDuration(strings.TrimSpace(v))
	return int64(d), err == nil
}

func parseNumber(v string) (int64, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	return int64(n), err == nil
}

func (f Field) Value(c Config) string { return f.get(&c) }

func (f Field) Default() string {
	d := Default()
	return f.get(&d)
}

func duration(key, doc string, ptr func(*Config) *time.Duration, lim limit, presets ...time.Duration) Field {
	lim.get = func(c Config) time.Duration { return *ptr(&c) }
	return Field{
		Key:     key,
		Doc:     doc,
		Choices: choices(presets, time.Duration.String),
		parse:   parseDuration,
		limit:   &lim,
		get:     func(c *Config) string { return ptr(c).String() },
		set: func(c *Config, v string) error {
			d, ok := parseDuration(v)
			if !ok {
				return fmt.Errorf("%s: %w: %q", key, ErrBadDuration, v)
			}
			*ptr(c) = time.Duration(d)
			return nil
		},
	}
}

func boolean(key, doc string, ptr func(*Config) *bool) Field {
	return Field{
		Key:     key,
		Doc:     doc,
		Choices: []string{"false", "true"},
		get:     func(c *Config) string { return strconv.FormatBool(*ptr(c)) },
		set: func(c *Config, v string) error {
			b, err := strconv.ParseBool(strings.TrimSpace(v))
			if err != nil {
				return fmt.Errorf("%s: %w: %q", key, ErrNotBool, v)
			}
			*ptr(c) = b
			return nil
		},
	}
}

func number(key, doc string, ptr func(*Config) *int, presets ...int) Field {
	return Field{
		Key:     key,
		Doc:     doc,
		Choices: choices(presets, strconv.Itoa),
		parse:   parseNumber,
		get:     func(c *Config) string { return strconv.Itoa(*ptr(c)) },
		set: func(c *Config, v string) error {
			n, ok := parseNumber(v)
			if !ok {
				return fmt.Errorf("%s: %w: %q", key, ErrNotNumber, v)
			}
			*ptr(c) = int(n)
			return nil
		},
	}
}

func text(key, doc string, ptr func(*Config) *string) Field {
	return Field{
		Key: key,
		Doc: doc,
		get: func(c *Config) string { return *ptr(c) },
		set: func(c *Config, v string) error {
			*ptr(c) = strings.TrimSpace(v)
			return nil
		},
	}
}

func choice[T ~string](key, doc string, all []string, ptr func(*Config) *T) Field {
	return Field{
		Key:     key,
		Doc:     doc + ", one of " + strings.Join(all, ", "),
		Choices: all,
		get:     func(c *Config) string { return string(*ptr(c)) },
		set: func(c *Config, v string) error {
			*ptr(c) = T(strings.TrimSpace(v))
			return nil
		},
	}
}

func staticFields() []Field {
	return []Field{
		duration("session.idle", "lock the vault after this long without use", func(c *Config) *time.Duration { return &c.Session.Idle },
			limit{max: maxSession, warn: time.Hour, risk: "the vault stays unlocked while you are away from the machine"},
			time.Minute, 5*time.Minute, 15*time.Minute, 30*time.Minute, time.Hour, 2*time.Hour, 4*time.Hour),
		duration("session.hard", "lock the vault this long after unlocking, whatever you do", func(c *Config) *time.Duration { return &c.Session.Hard },
			limit{max: maxSession, warn: 12 * time.Hour, risk: "one unlock keeps the vault open that long"},
			time.Hour, 2*time.Hour, 4*time.Hour, 8*time.Hour, 12*time.Hour, 24*time.Hour),
		duration("clipboard.clear", "wipe a copied secret from the clipboard after this long", func(c *Config) *time.Duration { return &c.Clipboard.Clear },
			limit{max: maxClip, warn: 10 * time.Minute, risk: "a copied secret sits on the clipboard that long"},
			10*time.Second, 20*time.Second, 30*time.Second, 45*time.Second, time.Minute, 2*time.Minute, 5*time.Minute),
		duration("sync.stale", "pull again when the last sync is older than this", func(c *Config) *time.Duration { return &c.Sync.Stale },
			limit{max: maxStale, warn: 7 * 24 * time.Hour, risk: "creds stops pulling remote changes on its own"},
			time.Minute, 5*time.Minute, 15*time.Minute, 30*time.Minute, time.Hour, 6*time.Hour, 24*time.Hour),
		choice("ui.mode", "how the browse screen fills the terminal", Modes(), func(c *Config) *Mode { return &c.UI.Mode }),
		boolean("ui.altscreen", "swap to a clean screen while the TUI runs, in either mode", func(c *Config) *bool { return &c.UI.AltScreen }),
		number("ui.height", "rows inline mode may use, between "+strconv.Itoa(MinHeight)+" and "+strconv.Itoa(MaxHeight), func(c *Config) *int { return &c.UI.Height },
			10, 12, 15, 20, 25, 30, 40, 50, 60),
		duration("sync.after", "wait this long after the last change before syncing", func(c *Config) *time.Duration { return &c.Sync.After },
			limit{max: maxAfter, warn: time.Minute, risk: "a change sits on this machine only for that long before it is pushed"},
			time.Second, 2*time.Second, 5*time.Second, 10*time.Second, 30*time.Second, time.Minute),
		duration("sync.every", "sync at most this often in the background", func(c *Config) *time.Duration { return &c.Sync.Every },
			limit{max: maxEvery, warn: 15 * time.Minute, risk: "a burst of changes waits that long for the next background sync"},
			10*time.Second, 30*time.Second, time.Minute, 5*time.Minute, 15*time.Minute),
		choice("sync.sign", "sign vault commits", Signs(), func(c *Config) *Sign { return &c.Sync.Sign }),
		text("sync.name", "name on vault commits, blank uses your global git user.name", func(c *Config) *string { return &c.Sync.Name }),
		text("sync.email", "email on vault commits, blank uses your global git user.email", func(c *Config) *string { return &c.Sync.Email }),
		text("sync.signkey", "signing key for sync.sign = ssh, blank uses your user.signingkey", func(c *Config) *string { return &c.Sync.SignKey }),
		number("vault.history", "keep this many previous versions of each entry, 0 turns it off", func(c *Config) *int { return &c.Vault.History },
			0, 1, 3, 5, 10, 20, 50),
		text("vault.machine", "name recorded on each edit, blank uses the hostname", func(c *Config) *string { return &c.Vault.Machine }),
		duration("device.max_age", "ask the master password again when the last one is older than this, 0 never asks", func(c *Config) *time.Duration { return &c.Device.MaxAge },
			limit{max: maxDevice, warn: 7 * 24 * time.Hour, risk: "a trusted device unlocks that long without the master password", zero: "a trusted device never asks for the master password again"},
			0, 24*time.Hour, 72*time.Hour, 7*24*time.Hour, 14*24*time.Hour, 30*24*time.Hour),
		choice("render.shell", "shell used to quote values", render.Shells(), func(c *Config) *render.Shell { return &c.Render.Shell }),
	}
}

func formatField(name string) Field {
	return Field{
		Key: FormatPrefix + name,
		Doc: "output template for " + name,
		get: func(c *Config) string { return c.Render.Formats[name] },
		set: func(c *Config, v string) error {
			if c.Render.Formats == nil {
				c.Render.Formats = map[string]string{}
			}
			c.Render.Formats[name] = v
			return nil
		},
		unset: func(c *Config) { delete(c.Render.Formats, name) },
	}
}

func Fields(c Config) []Field {
	fields := staticFields()
	for _, name := range slices.Sorted(maps.Keys(c.Render.Formats)) {
		fields = append(fields, formatField(name))
	}
	return fields
}

func lookup(key string) (Field, error) {
	key = strings.TrimSpace(key)
	for _, f := range staticFields() {
		if f.Key == key {
			return f, nil
		}
	}
	if name, ok := strings.CutPrefix(key, FormatPrefix); ok && name != "" {
		return formatField(name), nil
	}
	return Field{}, fmt.Errorf("%w: %s", ErrUnknownKey, key)
}

func Get(c Config, key string) (string, error) {
	f, err := lookup(key)
	if err != nil {
		return "", err
	}
	if name, ok := strings.CutPrefix(f.Key, FormatPrefix); ok {
		if _, set := c.Render.Formats[name]; !set {
			return "", fmt.Errorf("%w: %s", ErrUnknownKey, f.Key)
		}
	}
	return f.Value(c), nil
}

func Set(c Config, key, value string) (Config, error) {
	f, err := lookup(key)
	if err != nil {
		return c, err
	}
	c.Render.Formats = maps.Clone(c.Render.Formats)
	if err := f.set(&c, value); err != nil {
		return c, err
	}
	return c, c.validate()
}

func Next(key, current string, delta int) (string, error) {
	f, err := lookup(key)
	if err != nil {
		return "", err
	}
	n := len(f.Choices)
	if n == 0 {
		return "", fmt.Errorf("%w: %s", ErrNoChoices, f.Key)
	}
	if i := slices.Index(f.Choices, strings.TrimSpace(current)); i >= 0 {
		return f.Choices[((i+delta)%n+n)%n], nil
	}
	cur, ok := int64(0), false
	if f.parse != nil {
		cur, ok = f.parse(current)
	}
	if !ok || delta == 0 {
		return f.Choices[0], nil
	}
	if delta > 0 {
		for _, c := range f.Choices {
			if v, ok := f.parse(c); ok && v > cur {
				return c, nil
			}
		}
		return f.Choices[0], nil
	}
	for i := n - 1; i >= 0; i-- {
		if v, ok := f.parse(f.Choices[i]); ok && v < cur {
			return f.Choices[i], nil
		}
	}
	return f.Choices[n-1], nil
}

func Unset(c Config, key string) (Config, error) {
	f, err := lookup(key)
	if err != nil {
		return c, err
	}
	if f.unset == nil {
		return c, fmt.Errorf("%w: %s", ErrNotRemovable, f.Key)
	}
	c.Render.Formats = maps.Clone(c.Render.Formats)
	f.unset(&c)
	return c, c.validate()
}
