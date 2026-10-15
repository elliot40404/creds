package sshdir

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

var defaultKeys = []string{"id_rsa", "id_ecdsa", "id_ecdsa_sk", "id_ed25519", "id_ed25519_sk"}

type Usage struct {
	Used     []File
	Unused   []File
	Missing  []string
	Warnings []error
}

func ReadUsage(res Result, dir, home string) Usage {
	w := walker{res: res, dir: dir, home: home, seen: map[string]bool{}, named: map[string]bool{}}
	if f, ok := w.config("config"); ok {
		w.walk(f)
	}
	return w.split()
}

type walker struct {
	res   Result
	dir   string
	home  string
	seen  map[string]bool
	named map[string]bool
	order []string
	warns []error
}

func (w *walker) config(rel string) (File, bool) {
	for _, f := range w.res.Files {
		if f.Kind == KindConfig && f.Rel == rel {
			return f, true
		}
	}
	return File{}, false
}

func (w *walker) walk(f File) {
	done, seen := w.seen[f.Rel]
	if seen {
		if !done {
			w.warns = append(w.warns, fmt.Errorf("%s: %w", f.Rel, ErrIncludeLoop))
		}
		return
	}
	w.seen[f.Rel] = false
	for _, d := range parseConfig(f.Data) {
		w.apply(f.Rel, d)
	}
	w.seen[f.Rel] = true
}

func (w *walker) apply(rel string, d directive) {
	switch {
	case d.err != nil:
		w.warn(rel, d, "", d.err)
	case len(d.args) == 0:
		w.warn(rel, d, "", ErrSyntax)
	case d.key == keyIdentity:
		w.identity(rel, d)
	default:
		for _, p := range d.args {
			w.include(rel, d, p)
		}
	}
}

func (w *walker) identity(rel string, d directive) {
	v := d.args[0]
	if strings.EqualFold(v, "none") {
		return
	}
	target, err := w.resolve(v)
	if err != nil {
		w.warn(rel, d, v, err)
		return
	}
	if !w.named[target] {
		w.named[target] = true
		w.order = append(w.order, target)
	}
}

func (w *walker) include(rel string, d directive, pattern string) {
	target, err := w.resolve(pattern)
	if err != nil {
		w.warn(rel, d, pattern, err)
		return
	}
	for _, f := range w.res.Files {
		if f.Kind != KindConfig {
			continue
		}
		ok, err := path.Match(target, f.Rel)
		if err != nil {
			w.warn(rel, d, pattern, ErrSyntax)
			return
		}
		if ok {
			w.walk(f)
		}
	}
}

func (w *walker) resolve(v string) (string, error) {
	switch {
	case v == "":
		return "", ErrSyntax
	case strings.Contains(v, "%") || strings.Contains(v, "${"):
		return "", ErrToken
	}
	p := filepath.FromSlash(v)
	switch {
	case v == "~":
		p = w.home
	case strings.HasPrefix(v, "~") && os.IsPathSeparator(v[1]):
		p = filepath.Join(w.home, p[2:])
	case strings.HasPrefix(v, "~"):
		return "", ErrOutside
	case !filepath.IsAbs(p) && !os.IsPathSeparator(p[0]):
		p = filepath.Join(w.dir, p)
	}
	rel, err := filepath.Rel(w.dir, p)
	rel = filepath.ToSlash(rel)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", ErrOutside
	}
	return rel, nil
}

func (w *walker) warn(rel string, d directive, value string, err error) {
	if value != "" {
		err = fmt.Errorf("%s: %w", value, err)
	}
	w.warns = append(w.warns, fmt.Errorf("%s:%d: %w", rel, d.line, err))
}

func (w *walker) split() Usage {
	u := Usage{Warnings: w.warns}
	for _, k := range w.res.Keys() {
		if w.named[k.Rel] || slices.Contains(defaultKeys, k.Rel) {
			u.Used = append(u.Used, k)
		} else {
			u.Unused = append(u.Unused, k)
		}
	}
	for _, rel := range w.order {
		if !w.exists(rel) {
			u.Missing = append(u.Missing, rel)
		}
	}
	return u
}

func (w *walker) exists(rel string) bool {
	for _, f := range w.res.Files {
		if f.Rel == rel {
			return true
		}
	}
	for _, s := range w.res.Skipped {
		if s.Rel == rel {
			return true
		}
	}
	return false
}
