package app

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/sshdir"
)

var ErrSSHKey = errors.New("bad ssh key file")

func sshInside(dir, abs string) (string, bool) {
	rel, err := filepath.Rel(dir, abs)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

func insideSSHKey(res sshdir.Result, rel, name string) ([]sshdir.File, string, error) {
	if name != "" && name != rel {
		return nil, "", fmt.Errorf("%w: %s is already in ~/.ssh, --name works only for a key outside it", ErrSSHKey, rel)
	}
	key, err := scannedSSH(res, rel)
	if err != nil {
		return nil, "", sshReadErr(rel, err)
	}
	if key.Kind != sshdir.KindPrivateKey {
		return nil, "", fmt.Errorf("%w: %s is not a private key", ErrSSHKey, rel)
	}
	pub, err := scannedSSH(res, rel+".pub")
	files, note := withSSHPub(key, pub.Data, err)
	return files, note, nil
}

func scannedSSH(res sshdir.Result, rel string) (sshdir.File, error) {
	if i := slices.IndexFunc(res.Files, func(f sshdir.File) bool { return f.Rel == rel }); i >= 0 {
		return res.Files[i], nil
	}
	if i := slices.IndexFunc(res.Skipped, func(f sshdir.Skip) bool { return f.Rel == rel }); i >= 0 {
		return sshdir.File{}, res.Skipped[i].Err
	}
	return sshdir.File{}, fs.ErrNotExist
}

func outsideSSHKey(abs, name string) (sshdir.File, error) {
	if name == "" {
		name = filepath.Base(abs)
	}
	data, err := fsutil.ReadFileLimit(abs, sshdir.MaxFileSize)
	if err != nil {
		return sshdir.File{}, sshReadErr(abs, err)
	}
	if sshdir.Classify(name, data) != sshdir.KindPrivateKey {
		return sshdir.File{}, fmt.Errorf("%w: %s is not a private key", ErrSSHKey, abs)
	}
	if !sshNamePart.MatchString(name) || name == "config" || strings.HasSuffix(name, ".pub") {
		return sshdir.File{}, fmt.Errorf("%w: name %q, pass --name with letters, digits, . _ - and no .pub, like work_key", ErrSSHKey, name)
	}
	return sshdir.File{Rel: name, Kind: sshdir.KindPrivateKey, Data: data}, nil
}

func withSSHPub(key sshdir.File, data []byte, err error) ([]sshdir.File, string) {
	files := []sshdir.File{key}
	rel := key.Rel + ".pub"
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return files, ""
	case err != nil:
		return files, "left out " + rel + ": " + err.Error()
	case sshdir.Classify(rel, data) != sshdir.KindPublicKey || !sshdir.IsPublicKey(data):
		return files, "left out " + rel + ": not a public key"
	}
	return append(files, sshdir.File{Rel: rel, Kind: sshdir.KindPublicKey, Data: data}), ""
}

func sshReadErr(path string, err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%w: %s does not exist or is not a private key", ErrSSHKey, path)
	case errors.Is(err, fsutil.ErrTooLarge):
		return fmt.Errorf("%w: %s is over %d KiB", ErrSSHKey, path, sshdir.MaxFileSize>>10)
	}
	return fmt.Errorf("%w: %s: %s", ErrSSHKey, path, err.Error())
}
