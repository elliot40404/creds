package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

const (
	maxBinary   = 128 << 20
	maxUnpacked = 256 << 20
	exePerm     = 0o755
)

func BinaryName(goos string) string {
	if goos == "windows" {
		return "creds.exe"
	}
	return "creds"
}

func extract(archive, goos, dir string) (path string, err error) {
	in, err := os.Open(filepath.Clean(archive))
	if err != nil {
		return "", err
	}
	defer func() { _ = in.Close() }()
	out, err := os.CreateTemp(dir, tempNew+"*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(out.Name())
		}
	}()
	unpack := untar
	if goos == "windows" {
		unpack = unzip
	}
	if err = errors.Join(unpack(in, BinaryName(goos), out), finish(out)); err != nil {
		return "", err
	}
	return out.Name(), nil
}

func finish(f *os.File) error {
	var err error
	if runtime.GOOS != "windows" {
		err = f.Chmod(exePerm)
	}
	return errors.Join(err, f.Sync(), f.Close())
}

func untar(in *os.File, name string, w io.Writer) error {
	gz, err := gzip.NewReader(in)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadRelease, err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(io.LimitReader(gz, maxUnpacked))
	found := false
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return present(found)
		}
		if err != nil {
			return fmt.Errorf("%w: %w", ErrBadRelease, err)
		}
		hit, err := wanted(h.Name, name, found)
		if err != nil {
			return err
		}
		if !hit {
			continue
		}
		if err := regular(h.Typeflag == tar.TypeReg, h.Size); err != nil {
			return err
		}
		if err := copyCapped(w, tr); err != nil {
			return err
		}
		found = true
	}
}

func unzip(in *os.File, name string, w io.Writer) error {
	st, err := in.Stat()
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(in, st.Size())
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadRelease, err)
	}
	var file *zip.File
	for _, f := range zr.File {
		hit, err := wanted(f.Name, name, file != nil)
		if err != nil {
			return err
		}
		if hit {
			file = f
		}
	}
	if err := present(file != nil); err != nil {
		return err
	}
	if err := regular(file.Mode().IsRegular(), int64(min(file.UncompressedSize64, maxBinary+1))); err != nil {
		return err
	}
	r, err := file.Open()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadRelease, err)
	}
	defer func() { _ = r.Close() }()
	return copyCapped(w, r)
}

func wanted(entry, name string, found bool) (bool, error) {
	if !filepath.IsLocal(entry) {
		return false, fmt.Errorf("%w: unsafe archive entry %q", ErrBadRelease, entry)
	}
	if filepath.Clean(entry) != name {
		return false, nil
	}
	if found {
		return false, fmt.Errorf("%w: %s twice in the archive", ErrBadRelease, name)
	}
	return true, nil
}

func regular(ok bool, size int64) error {
	if !ok {
		return fmt.Errorf("%w: the binary is not a regular file", ErrBadRelease)
	}
	if size > maxBinary {
		return fmt.Errorf("%w: the binary is larger than %d bytes", ErrBadRelease, maxBinary)
	}
	return nil
}

func present(found bool) error {
	if !found {
		return fmt.Errorf("%w: no creds binary in the archive", ErrBadRelease)
	}
	return nil
}

func copyCapped(w io.Writer, r io.Reader) error {
	n, err := io.Copy(w, io.LimitReader(r, maxBinary+1))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadRelease, err)
	}
	if n > maxBinary {
		return fmt.Errorf("%w: the binary is larger than %d bytes", ErrBadRelease, maxBinary)
	}
	return nil
}
