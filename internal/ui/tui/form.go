package tui

import (
	"errors"
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/vault"
)

type rowKind int

const (
	kindField rowKind = iota
	kindPath
	kindEngine
	kindConn
	kindParam
)

const (
	rowPath   = "path"
	rowEngine = "engine"
	rowConn   = "connection string"
)

type formRow struct {
	kind   rowKind
	key    string
	input  textInput
	custom bool
	keep   bool
	ghost  string
	err    string
}

type formScreen struct {
	picking      bool
	edit         bool
	orig         vault.Entry
	typ          vault.Type
	types        []vault.Type
	cursor       int
	engines      []string
	engine       int
	engineLocked bool
	engineNote   string
	rows         []formRow
	removed      []string
	focus        int
	paths        []string
	cands        []string
	naming       *textInput
	nameErr      string
	errMsg       string
	saving       bool
}

func DefaultHooks() Hooks {
	return Hooks{Add: NewAddForm, Edit: NewEditForm, Prompt: NewPrompt}
}

func NewAddForm(paths []string) Screen {
	return formScreen{picking: true, types: app.Types(), engines: app.Engines(), paths: paths, cands: search.Candidates(paths)}
}

func NewEditForm(orig vault.Entry) Screen {
	f := formScreen{edit: true, orig: orig, typ: orig.Type}
	f.rows = []formRow{{kind: kindPath, key: rowPath, input: newInput(orig.Path, false)}}
	for _, s := range app.FieldSpecs(orig.Type) {
		v, _ := app.Lookup(orig, s.Key)
		f.rows = append(f.rows, existingRow(s.Key, v, s.Secret, false))
	}
	for _, fd := range orig.Fields {
		if !app.IsSpecField(orig.Type, fd.Name) {
			f.rows = append(f.rows, existingRow(fd.Name, fd.Value, fd.Secret, true))
		}
	}
	f.rows = append(f.rows, paramRows(orig.Params)...)
	return f
}

func existingRow(key, value string, secret, custom bool) formRow {
	if secret {
		return formRow{key: key, input: newInput("", true), custom: custom, keep: value != ""}
	}
	return formRow{key: key, input: newInput(value, false), custom: custom}
}

func (f formScreen) pick(t vault.Type) formScreen {
	f.picking, f.typ = false, t
	f.rows = []formRow{{kind: kindPath, key: rowPath}}
	if t == vault.TypeDatabase {
		f.rows = append(f.rows, formRow{kind: kindEngine, key: rowEngine}, formRow{kind: kindConn, key: rowConn, input: newInput("", true)})
		return f
	}
	for _, s := range app.FieldSpecs(t) {
		f.rows = append(f.rows, formRow{key: s.Key, input: newInput("", s.Secret)})
	}
	return f
}

func (f formScreen) typing() bool {
	return !f.picking
}

func (f formScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case unmaskedMsg:
		return f.unmask(msg), nil
	case FormError:
		f.saving = false
		f.errMsg = saveHint(msg.Err)
		return f, nil
	case tea.KeyPressMsg:
		if f.picking {
			return f.pickKey(msg)
		}
		if f.naming != nil {
			return f.nameKey(msg), nil
		}
		return f.fieldKey(msg)
	case tea.PasteMsg:
		if f.naming != nil {
			name := *f.naming
			name.update(msg)
			f.naming = &name
		} else if !f.picking {
			f = f.edited(msg)
		}
	}
	return f, nil
}

func (f formScreen) pickKey(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	if i, ok := pickNumber(key, len(f.types)); ok {
		return f.pick(f.types[i]), nil
	}
	if dy := navDelta(key); dy != 0 {
		f.cursor = moveCursor(f.cursor, dy, len(f.types))
		return f, nil
	}
	switch key.String() {
	case "enter":
		return f.pick(f.types[f.cursor]), nil
	case "esc", "q":
		return f, send(FormResult{Canceled: true})
	}
	return f, nil
}

func (f formScreen) fieldKey(key tea.KeyPressMsg) (Screen, tea.Cmd) {
	if f.saving {
		return f, nil
	}
	switch key.String() {
	case "esc":
		return f, send(FormResult{Canceled: true})
	case "tab":
		if f.complete() {
			return f, nil
		}
		f.focus = (f.focus + 1) % len(f.rows)
	case "down":
		f.focus = (f.focus + 1) % len(f.rows)
	case "shift+tab", "up":
		f.focus = (f.focus + len(f.rows) - 1) % len(f.rows)
	case "enter":
		if f.focus < len(f.rows)-1 {
			f.focus++
			return f, nil
		}
		return f.submit()
	case "ctrl+s":
		return f.submit()
	case "ctrl+n":
		f.naming, f.nameErr = &textInput{}, ""
	case "ctrl+t":
		return f.toggleKey()
	case "ctrl+d":
		f = f.removeRow()
	case "ctrl+r":
		return f.revealKey()
	case "left", "right":
		f.cycleEngine(key.String())
	default:
		f = f.edited(key)
	}
	return f, nil
}

func (f formScreen) edited(msg tea.Msg) formScreen {
	f.rows = slices.Clone(f.rows)
	r := &f.rows[f.focus]
	if r.kind != kindEngine && r.input.update(msg) {
		r.err, f.errMsg = "", ""
	}
	f.suggest()
	f.detectEngine()
	return f
}

func (f *formScreen) detectEngine() {
	if f.rows[f.focus].kind != kindConn || f.engineLocked {
		return
	}
	found, ok := render.EngineFor(f.rows[f.focus].input.String())
	if !ok {
		return
	}
	i := slices.Index(f.engines, found)
	if i < 0 || i == f.engine {
		return
	}
	f.engine, f.engineNote = i, "set from the connection string"
}

func (f *formScreen) suggest() {
	for i := range f.rows {
		r := &f.rows[i]
		if r.kind != kindPath {
			continue
		}
		r.ghost, r.err = "", ""
		if f.edit || len(f.cands) == 0 {
			continue
		}
		typed := r.input.String()
		if slices.Contains(f.paths, typed) {
			r.err = saveHint(fmt.Errorf("%w: %s", vault.ErrDuplicatePath, typed))
			continue
		}
		if filled, _ := search.Complete(f.cands, typed); len(filled) > len(typed) {
			r.ghost = filled[len(typed):]
		}
	}
}

func (f *formScreen) complete() bool {
	r := &f.rows[f.focus]
	if r.kind != kindPath || r.ghost == "" {
		return false
	}
	f.rows = slices.Clone(f.rows)
	f.rows[f.focus].input.insert(r.ghost)
	f.suggest()
	return true
}

func (f *formScreen) cycleEngine(dir string) {
	if f.rows[f.focus].kind != kindEngine {
		return
	}
	f.engineLocked, f.engineNote = true, ""
	n := len(f.engines)
	if dir == "left" {
		f.engine = (f.engine + n - 1) % n
	} else {
		f.engine = (f.engine + 1) % n
	}
}

func (f formScreen) nameKey(key tea.KeyPressMsg) formScreen {
	switch key.String() {
	case "esc":
		f.naming = nil
		return f
	case "enter":
		return f.addCustom()
	}
	name := *f.naming
	name.update(key)
	f.naming, f.nameErr = &name, ""
	return f
}

func (f formScreen) addCustom() formScreen {
	name := f.naming.String()
	if err := f.checkName(name); err != "" {
		f.nameErr = err
		return f
	}
	f.rows = append(slices.Clone(f.rows), formRow{key: name, input: newInput("", false), custom: true})
	f.focus, f.naming = len(f.rows)-1, nil
	return f
}

func (f formScreen) checkName(name string) string {
	if name == "" {
		return "field name is empty. type a name like pin, or press esc"
	}
	taken := name == rowPath || app.BuiltinField(&vault.Entry{}, name) != nil ||
		slices.ContainsFunc(f.rows, func(r formRow) bool { return r.key == name })
	if taken {
		return "field " + name + " already exists. pick another name"
	}
	return ""
}

func saveHint(err error) string {
	msg := firstLine(err).Error()
	switch {
	case errors.Is(err, vault.ErrDuplicatePath):
		return msg + ". pick another path and press ctrl+s"
	case errors.Is(err, vault.ErrInvalidEntry):
		return msg + ". fix the fields and press ctrl+s"
	}
	return "could not save: " + msg + ". press ctrl+s to retry or esc to cancel"
}
