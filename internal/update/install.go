package update

import (
	"cmp"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

var ErrDevBuild = errors.New("update: this creds was not built by the release workflow")

func Exe() (string, error) {
	p, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(p)
}

func (c Checker) Install(ctx context.Context, rel Release, exe string) error {
	goos, goarch := cmp.Or(c.OS, runtime.GOOS), cmp.Or(c.Arch, runtime.GOARCH)
	dir := filepath.Dir(exe)
	archive, err := c.download(ctx, rel, AssetName(rel.Tag, goos, goarch), dir)
	if err != nil {
		return err
	}
	bin, err := extract(archive, goos, dir)
	_ = os.Remove(archive)
	if err != nil {
		return err
	}
	if err := swap(bin, exe); err != nil {
		_ = os.Remove(bin)
		return err
	}
	return nil
}
