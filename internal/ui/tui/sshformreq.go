package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/elliot40404/creds/internal/app"
)

const (
	keygenToggle = 2
	keygenPass   = 3
	keygenRepeat = 4
)

func newHostForm(keys []string) sshForm {
	key := "optional, a private key file in ~/.ssh"
	if len(keys) > 0 {
		key = "optional, like " + strings.Join(keys[:min(len(keys), 3)], ", ")
	}
	return sshForm{kind: sshFormHost, title: "Add ssh host", fields: []sshField{
		{label: "name", hint: "group/name, like beam/data_new"},
		{label: "host", hint: "host name or IP"},
		{label: "user", hint: "optional"},
		{label: "port", hint: "22 when blank"},
		{label: "key", hint: key},
	}}
}

func newKeyForm() sshForm {
	return sshForm{kind: sshFormKey, title: "Add ssh key", fields: []sshField{
		{label: "file", hint: "path to a private key file"},
		{label: "name", hint: "optional, name in ~/.ssh"},
	}}
}

func newKeygenForm() sshForm {
	return sshForm{kind: sshFormKeygen, title: "New ssh key", fields: []sshField{
		{label: "name", hint: "like id_work"},
		{label: "comment", hint: "optional, the name when blank"},
		{label: "use a passphrase", toggle: true},
	}}
}

func (f sshForm) flip() sshForm {
	f.wipe()
	f.fields[f.focus].on = !f.fields[f.focus].on
	f.fields = f.fields[:keygenPass]
	if f.fields[keygenToggle].on {
		f.fields = append(f.fields,
			sshField{label: "passphrase", input: newInput("", true)},
			sshField{label: "repeat", input: newInput("", true)})
	}
	f.errMsg, f.fix = "", ""
	return f
}

func (f *sshForm) request() (tea.Msg, bool) {
	switch f.kind {
	case sshFormHost:
		return f.hostRequest()
	case sshFormKey:
		return sshAddKeyMsg{file: homePath(f.val(0)), name: f.val(1)}, f.need(0, "file is required, the path to a private key file")
	}
	return f.keygenRequest()
}

func (f *sshForm) hostRequest() (tea.Msg, bool) {
	h := app.SSHHost{Name: f.val(0), Host: f.val(1), User: f.val(2), Key: f.val(4)}
	if !f.need(0, "name is required, like beam/data_new") || !f.need(1, "host is required, a host name or IP") {
		return nil, false
	}
	if p := f.val(3); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil {
			f.focus, f.errMsg, f.fix = 3, "port must be a number from 1 to 65535", ""
			return nil, false
		}
		h.Port = n
	}
	return sshAddHostMsg{host: h}, true
}

func (f *sshForm) keygenRequest() (tea.Msg, bool) {
	k := app.SSHKeygen{Name: f.val(0), Comment: f.val(1), Passphrase: f.fields[keygenToggle].on}
	if !f.need(0, "name is required, like id_work") {
		return nil, false
	}
	if !k.Passphrase {
		return sshKeygenMsg{gen: k}, true
	}
	pass, again := f.fields[keygenPass].input.value, f.fields[keygenRepeat].input.value
	switch {
	case len(pass) == 0:
		f.focus, f.errMsg, f.fix = keygenPass, "passphrase is empty, type it twice or switch it off", ""
		return nil, false
	case !slices.Equal(pass, again):
		f.wipe()
		f.focus, f.errMsg, f.fix = keygenPass, "passphrases do not match, type them again", ""
		return nil, false
	}
	k.PassphraseBytes = runeBytes(pass)
	f.wipe()
	return sshKeygenMsg{gen: k}, true
}

func runeBytes(rs []rune) []byte {
	n := 0
	for _, r := range rs {
		n += utf8.RuneLen(r)
	}
	b := make([]byte, 0, n)
	for _, r := range rs {
		b = utf8.AppendRune(b, r)
	}
	return b
}

func homePath(p string) string {
	rest, ok := strings.CutPrefix(p, "~")
	if !ok || (rest != "" && rest[0] != '/' && rest[0] != filepath.Separator) {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return p
	}
	return filepath.Join(home, rest)
}
