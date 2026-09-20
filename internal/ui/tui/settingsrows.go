package tui

import (
	"strings"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/safetext"
)

type settingKind int

const (
	settingConfig settingKind = iota
	settingRemote
	settingTrust
)

const (
	remoteKey   = "sync.remote"
	trustKey    = "trusted file"
	noneValue   = "none"
	remoteDoc   = "git url creds syncs the vault with, enter to change, x to remove"
	trustDoc    = "project file allowed to pick secrets, x to forget"
	previewHint = "new format override"

	remoteEditHint = "type the whole url, login included, empty keeps the current one"
)

type settingRow struct {
	kind    settingKind
	key     string
	value   string
	raw     string
	def     string
	derived string
	source  string
	doc     string
	opts    []string
}

func buildRows(c ConfigBackend) []settingRow {
	var rows []settingRow
	for _, f := range c.ConfigFields() {
		rows = append(rows, settingRow{
			kind: settingConfig, key: f.Key, value: f.Value, def: f.Default,
			derived: f.Derived, source: f.Source, doc: f.Doc, opts: f.Choices,
		})
	}
	rows = append(rows, remoteRow(c))
	files, err := c.TrustList()
	if err != nil {
		return rows
	}
	for _, f := range files {
		rows = append(rows, settingRow{kind: settingTrust, key: trustKey, value: f, doc: trustDoc})
	}
	return rows
}

func remoteRow(c ConfigBackend) settingRow {
	url := c.RemoteURL()
	if url == "" {
		return settingRow{kind: settingRemote, key: remoteKey, value: noneValue, def: noneValue, doc: remoteDoc}
	}
	return settingRow{kind: settingRemote, key: remoteKey, value: safetext.Remote(url), raw: url, def: noneValue, doc: remoteDoc}
}

func (r settingRow) editable() bool {
	return r.kind != settingTrust && !r.cycles()
}

func (r settingRow) cycles() bool {
	return r.kind == settingConfig && len(r.opts) > 0
}

func (r settingRow) removable() bool {
	switch r.kind {
	case settingRemote:
		return r.value != noneValue
	case settingTrust:
		return true
	case settingConfig:
		return strings.HasPrefix(r.key, config.FormatPrefix)
	}
	return false
}

func (r settingRow) editValue() string {
	if r.kind == settingRemote {
		return ""
	}
	return r.value
}

func (r settingRow) submitValue(typed string) string {
	if r.kind == settingRemote && typed == r.value && r.raw != "" {
		return r.raw
	}
	return typed
}

func pathNote(p app.PathInfo) string {
	if !p.Exists {
		return "missing"
	}
	return p.Note
}
