package envfile

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"unicode/utf8"

	"github.com/elliot40404/creds/internal/fsutil"
)

const (
	trustVersion = 1
	trustLimit   = 1 << 20
)

var ErrTrustFile = errors.New("invalid trust file")

type trustFile struct {
	Version  int               `json:"version"`
	Projects map[string]string `json:"projects"`
}

func (p Project) Refs() []string {
	var out []string
	if p.Env != "" {
		out = append(out, "all fields of "+p.Env)
	}
	for _, k := range slices.Sorted(maps.Keys(p.Map)) {
		line := k + " = " + p.Map[k].String()
		if risky(k) {
			line += riskyNote
		}
		out = append(out, line)
	}
	return out
}

func (r Ref) String() string {
	if r.Format != "" {
		return r.Path + "|" + r.Format
	}
	return r.Path + ":" + r.Field
}

func Trusted(store string, p Project) (bool, error) {
	f, err := loadTrust(store)
	if err != nil {
		return false, err
	}
	return p.Sum != "" && f.Projects[p.File] == p.Sum, nil
}

func Trust(store string, p Project) error {
	if !utf8.ValidString(p.File) {
		return errors.New("project file path is not valid UTF-8 text, rename the directory")
	}
	f, err := loadTrust(store)
	if err != nil {
		return err
	}
	f.Projects[p.File] = p.Sum
	return fsutil.WriteJSON(store, f)
}

func loadTrust(store string) (trustFile, error) {
	var f trustFile
	err := fsutil.ReadJSON(store, trustLimit, &f)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return trustFile{Version: trustVersion, Projects: map[string]string{}}, nil
	case errors.Is(err, fsutil.ErrTooLarge):
		return trustFile{}, fmt.Errorf("%w: too large", ErrTrustFile)
	case err != nil, f.Version != trustVersion:
		return trustFile{}, fmt.Errorf("%w: %s", ErrTrustFile, store)
	case f.Projects == nil:
		f.Projects = map[string]string{}
	}
	return f, nil
}

func sum(src string) string {
	h := sha256.Sum256([]byte(src))
	return hex.EncodeToString(h[:])
}

func TrustList(store string) ([]string, error) {
	f, err := loadTrust(store)
	if err != nil {
		return nil, err
	}
	return slices.Sorted(maps.Keys(f.Projects)), nil
}

func Untrust(store, file string) (bool, error) {
	f, err := loadTrust(store)
	if err != nil {
		return false, err
	}
	if _, ok := f.Projects[file]; !ok {
		return false, nil
	}
	delete(f.Projects, file)
	return true, fsutil.WriteJSON(store, f)
}
