package cli

import (
	"encoding/json/v2"
	"fmt"
	"io"
	"maps"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/vault"
)

type jsonField struct {
	Name   string  `json:"name"`
	Value  *string `json:"value,omitzero"`
	Secret bool    `json:"secret"`
}

type jsonEntry struct {
	Path     string            `json:"path"`
	Type     vault.Type        `json:"type"`
	Name     string            `json:"name,omitzero"`
	Username string            `json:"username,omitzero"`
	Host     string            `json:"host,omitzero"`
	URL      string            `json:"url,omitzero"`
	Notes    string            `json:"notes,omitzero"`
	Tags     []string          `json:"tags"`
	Fields   []jsonField       `json:"fields"`
	Params   map[string]string `json:"params,omitzero"`
	Created  time.Time         `json:"created"`
	Updated  time.Time         `json:"updated"`
}

type jsonSummary struct {
	Path     string     `json:"path"`
	Type     vault.Type `json:"type"`
	Name     string     `json:"name,omitzero"`
	Host     string     `json:"host,omitzero"`
	Username string     `json:"username,omitzero"`
	Engine   string     `json:"engine,omitzero"`
	Tags     []string   `json:"tags"`
	Created  time.Time  `json:"created"`
	Updated  time.Time  `json:"updated"`
}

type jsonValue struct {
	Path   string `json:"path"`
	Field  string `json:"field,omitzero"`
	Format string `json:"format,omitzero"`
	Value  string `json:"value"`
}

type jsonFormats struct {
	Path    string   `json:"path"`
	Formats []string `json:"formats"`
}

type jsonStatus struct {
	Remote       string    `json:"remote"`
	Ahead        int       `json:"ahead"`
	Behind       int       `json:"behind"`
	LastSync     time.Time `json:"last_sync,omitzero"`
	LastResult   string    `json:"last_result,omitzero"`
	LastError    string    `json:"last_error,omitzero"`
	LastErrorRaw string    `json:"last_error_raw,omitzero"`
	Conflicts    []string  `json:"conflicts"`
	Trusted      string    `json:"trusted,omitzero"`
	Generation   uint64    `json:"generation,omitzero"`
}

type jsonNote struct {
	Note string `json:"note"`
}

type jsonSync struct {
	Result    string   `json:"result"`
	Conflicts []string `json:"conflicts"`
}

type jsonTrust struct {
	File string   `json:"file"`
	Refs []string `json:"refs"`
}

func (e Env) jsonMode() bool {
	return e.json != nil && *e.json
}

func (e Env) writeJSON(v any) error {
	return writeJSONLine(e.Out, v)
}

func writeJSONLine(w io.Writer, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, escapeJSON(string(b)))
	return err
}

func escapeJSON(s string) string {
	var b strings.Builder
	for _, r := range s {
		if !safetext.IsControl(r) {
			b.WriteRune(r)
			continue
		}
		if r1, r2 := utf16.EncodeRune(r); r1 != utf8.RuneError {
			fmt.Fprintf(&b, `\u%04x\u%04x`, r1, r2)
			continue
		}
		fmt.Fprintf(&b, `\u%04x`, r)
	}
	return b.String()
}

func jsonParams(params map[string]string, show bool) map[string]string {
	if show {
		return params
	}
	out := maps.Clone(params)
	maps.DeleteFunc(out, func(k, _ string) bool { return render.SecretParam(k) })
	return out
}

func toJSONEntry(e vault.Entry, show bool) jsonEntry {
	out := jsonEntry{
		Path: e.Path, Type: e.Type, Name: e.Name, Username: e.Username, Host: e.Host, URL: e.URL, Notes: e.Notes,
		Tags: nonNil(e.Tags), Fields: make([]jsonField, len(e.Fields)), Params: jsonParams(e.Params, show), Created: e.Created, Updated: e.Updated,
	}
	for i, f := range e.Fields {
		out.Fields[i] = jsonField{Name: f.Name, Secret: f.Secret}
		if !f.Secret || show {
			out.Fields[i].Value = &f.Value
		}
	}
	return out
}

func toJSONSummaries(items []search.Summary) map[string][]jsonSummary {
	out := make([]jsonSummary, len(items))
	for i, it := range items {
		out[i] = jsonSummary{Path: it.Path, Type: it.Type, Name: it.Name, Host: it.Host, Username: it.Username, Engine: it.Engine, Tags: nonNil(it.Tags), Created: it.Created, Updated: it.Updated}
	}
	return map[string][]jsonSummary{"entries": out}
}

func toJSONStatus(st app.SyncStatus) jsonStatus {
	return jsonStatus{
		Remote: safetext.Remote(st.Remote), Ahead: st.Ahead, Behind: st.Behind, LastSync: st.LastSync,
		LastResult: st.LastResult, LastError: plainSyncError(st.LastError), LastErrorRaw: st.LastError, Conflicts: nonNil(st.Conflicts),
		Trusted: st.Trusted, Generation: st.Generation,
	}
}

func toJSONEnv(vars []envfile.Var) map[string]string {
	out := make(map[string]string, len(vars))
	for _, v := range vars {
		out[v.Key] = v.Value
	}
	return out
}

func toJSONTrust(p envfile.Project) jsonTrust {
	return jsonTrust{File: p.File, Refs: nonNil(p.Refs())}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
