package render

import (
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"text/template"
)

const (
	MaxOutput = 64 << 10
	MaxSource = 8 << 10
)

var (
	ErrUnknownFormat = errors.New("render: unknown format")
	ErrBadFormatKey  = errors.New("render: format key must be engine.format")
	ErrTooLong       = errors.New("render: template is too long")
	ErrTooBig        = errors.New("render: output is too big")
)

type Renderer struct {
	tmpl  map[string]map[string]*template.Template
	funcs template.FuncMap
}

func New(overrides map[string]string, shell Shell) (*Renderer, error) {
	shell, err := ParseShell(string(shell))
	if err != nil {
		return nil, err
	}
	funcs := shell.funcs()
	funcs["pathesc"] = url.PathEscape
	funcs["queryesc"] = url.QueryEscape
	r := &Renderer{tmpl: make(map[string]map[string]*template.Template, len(builtins)), funcs: funcs}
	for engine, formats := range builtins {
		r.tmpl[engine] = make(map[string]*template.Template, len(formats))
		for name, src := range formats {
			if err := r.add(engine, name, src); err != nil {
				return nil, err
			}
		}
	}
	for key, src := range overrides {
		engine, name, ok := strings.Cut(key, ".")
		if !ok || name == "" {
			return nil, fmt.Errorf("%w: %q", ErrBadFormatKey, key)
		}
		if _, ok := r.tmpl[engine]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownEngine, engine)
		}
		if err := r.add(engine, name, src); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Renderer) add(engine, name, src string) error {
	if len(src) > MaxSource {
		return fmt.Errorf("%w: %s.%s is %d bytes, the limit is %d", ErrTooLong, engine, name, len(src), MaxSource)
	}
	t, err := template.New(engine + "." + name).Option("missingkey=error").Funcs(r.funcs).Parse(src)
	if err != nil {
		return fmt.Errorf("render: %w", err)
	}
	r.tmpl[engine][name] = t
	return nil
}

func (r *Renderer) Formats(engine string) ([]string, error) {
	formats, ok := r.tmpl[engine]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownEngine, engine)
	}
	return slices.Sorted(maps.Keys(formats)), nil
}

func (r *Renderer) Render(engine, format string, c Conn) (string, error) {
	formats, ok := r.tmpl[engine]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownEngine, engine)
	}
	t, ok := formats[format]
	if !ok {
		return "", fmt.Errorf("%w: %s.%s", ErrUnknownFormat, engine, format)
	}
	w := &capped{limit: MaxOutput}
	if err := t.Execute(w, c.data(engine)); err != nil {
		if errors.Is(err, ErrTooBig) {
			return "", ErrTooBig
		}
		return "", fmt.Errorf("render: %w", err)
	}
	return strings.TrimSuffix(w.b.String(), "\n"), nil
}

func (c Conn) data(engine string) map[string]any {
	scheme := c.Scheme
	if scheme == "" {
		scheme = defaultScheme[engine]
	}
	params := c.Params
	if params == nil {
		params = map[string]string{}
	}
	return map[string]any{
		"scheme":   scheme,
		"host":     c.Host,
		"port":     c.Port,
		"username": c.Username,
		"password": c.Password,
		"database": c.Database,
		"params":   params,
		"url":      buildURL(scheme, c),
	}
}

type capped struct {
	b     strings.Builder
	limit int
}

func (c *capped) Write(p []byte) (int, error) {
	if c.b.Len()+len(p) > c.limit {
		return 0, ErrTooBig
	}
	return c.b.Write(p)
}
