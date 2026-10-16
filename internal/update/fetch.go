package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	DownloadTimeout = 5 * time.Minute
	SumsName        = "checksums.txt"
	maxArchive      = 64 << 20
	maxSums         = 64 << 10
	maxRedirects    = 10
)

var (
	ErrChecksum = errors.New("update: download does not match checksums.txt")
	ErrNoAsset  = errors.New("update: the release has no file for this system")
)

func AssetName(tag, goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return "creds_" + strings.TrimPrefix(tag, "v") + "_" + goos + "_" + goarch + ext
}

func (r Release) asset(name string) (Asset, error) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("%w: %s", ErrNoAsset, name)
}

func (c Checker) download(ctx context.Context, rel Release, name, dir string) (path string, err error) {
	a, err := rel.asset(name)
	if err != nil {
		return "", err
	}
	if a.Size <= 0 || a.Size > maxArchive {
		return "", fmt.Errorf("%w: %s has size %d", ErrBadRelease, name, a.Size)
	}
	want, err := c.sum(ctx, rel, name)
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, ".creds-download-*")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(f.Name())
		}
	}()
	h := sha256.New()
	err = errors.Join(c.fetch(ctx, a.URL, a.Size, io.MultiWriter(f, h)), f.Close())
	if err != nil {
		return "", err
	}
	if subtle.ConstantTimeCompare(h.Sum(nil), want) != 1 {
		return "", fmt.Errorf("%w: %s", ErrChecksum, name)
	}
	return f.Name(), nil
}

func (c Checker) sum(ctx context.Context, rel Release, name string) ([]byte, error) {
	a, err := rel.asset(SumsName)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := c.fetch(ctx, a.URL, maxSums, &buf); err != nil {
		return nil, err
	}
	return parseSums(buf.String(), name)
}

func (c Checker) fetch(ctx context.Context, raw string, limit int64, w io.Writer) error {
	if err := httpsOnly(raw); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrBadRelease, err)
	}
	req.Header.Set("User-Agent", "creds/"+c.Version)
	resp, err := c.downloader().Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: download status %d", ErrBadRelease, resp.StatusCode)
	}
	n, err := io.Copy(w, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	if n > limit {
		return fmt.Errorf("%w: download larger than %d bytes", ErrBadRelease, limit)
	}
	return nil
}

func (c Checker) downloader() *http.Client {
	cl := http.Client{Timeout: DownloadTimeout}
	if c.Download != nil {
		cl = *c.Download
	}
	cl.CheckRedirect = redirect
	return &cl
}

func redirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("%w: too many redirects", ErrBadRelease)
	}
	return httpsOnly(req.URL.String())
}

func httpsOnly(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("%w: %q is not an https url", ErrBadRelease, raw)
	}
	return nil
}
