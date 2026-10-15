package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/elliot40404/creds/internal/safetext"
	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/vault"
)

const (
	FieldContent = "content"
	SSHPrefix    = "ssh/"
)

var (
	ErrSSHPath = errors.New("bad ssh file path")
	ErrSSHFile = errors.New("not an ssh file")
)

func SSHVaultPath(rel string) (string, error) {
	if err := checkSSHRel(rel); err != nil {
		return "", err
	}
	return SSHPrefix + rel, nil
}

func SSHRel(vaultPath string) (string, error) {
	rel, ok := strings.CutPrefix(vaultPath, SSHPrefix)
	if !ok {
		return "", fmt.Errorf("%w %q: not under %s", ErrSSHPath, vaultPath, SSHPrefix)
	}
	if err := checkSSHRel(rel); err != nil {
		return "", err
	}
	return rel, nil
}

func checkSSHRel(rel string) error {
	switch {
	case rel == "":
		return fmt.Errorf("%w: empty", ErrSSHPath)
	case strings.ContainsAny(rel, `\:`) || safetext.HasControl(rel):
		return fmt.Errorf("%w %q: has a \\, : or control character", ErrSSHPath, rel)
	case !filepath.IsLocal(filepath.FromSlash(rel)):
		return fmt.Errorf("%w %q: not a plain path inside the ssh dir", ErrSSHPath, rel)
	}
	if err := vault.ValidatePath(SSHPrefix + rel); err != nil {
		return fmt.Errorf("%w: %s", ErrSSHPath, err.Error())
	}
	return nil
}

func SSHEntry(f sshdir.File) (vault.Entry, error) {
	p, err := SSHVaultPath(f.Rel)
	if err != nil {
		return vault.Entry{}, err
	}
	if !utf8.Valid(f.Data) {
		return vault.Entry{}, fmt.Errorf("%w %q: not UTF-8 text", ErrSSHFile, f.Rel)
	}
	secret := f.Kind == sshdir.KindPrivateKey
	return vault.Entry{
		Path:   p,
		Type:   vault.TypeSSHFile,
		Fields: []vault.Field{{Name: FieldContent, Value: string(f.Data), Secret: secret}},
	}, nil
}

func SSHFile(e vault.Entry) (sshdir.File, error) {
	if e.Type != vault.TypeSSHFile {
		return sshdir.File{}, fmt.Errorf("%w %q: type %s", ErrSSHFile, e.Path, e.Type)
	}
	rel, err := SSHRel(e.Path)
	if err != nil {
		return sshdir.File{}, err
	}
	data, err := sshContent(e)
	if err != nil {
		return sshdir.File{}, err
	}
	kind := sshdir.Classify(rel, data)
	if kind == 0 {
		return sshdir.File{}, fmt.Errorf("%w %q: not a config, key or .pub file", ErrSSHFile, e.Path)
	}
	return sshdir.File{Rel: rel, Kind: kind, Data: data}, nil
}

func sshContent(e vault.Entry) ([]byte, error) {
	i := slices.IndexFunc(e.Fields, func(f vault.Field) bool { return f.Name == FieldContent })
	switch {
	case i < 0:
		return nil, fmt.Errorf("%w %q: no %s field", ErrSSHFile, e.Path, FieldContent)
	case len(e.Fields[i].Value) > sshdir.MaxFileSize:
		return nil, fmt.Errorf("%w %q: over %d bytes", ErrSSHFile, e.Path, sshdir.MaxFileSize)
	}
	return []byte(e.Fields[i].Value), nil
}
