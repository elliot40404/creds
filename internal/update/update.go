package update

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

const (
	Repo     = "elliot40404/creds"
	API      = "https://api.github.com"
	Timeout  = 5 * time.Second
	maxBody  = 256 << 10
	mimeJSON = "application/vnd.github+json"
)

var (
	ErrUnreachable = errors.New("update: could not reach GitHub")
	ErrRateLimited = errors.New("update: GitHub rate limit reached")
	ErrBadRelease  = errors.New("update: unexpected release data")
)

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type Release struct {
	Tag        string  `json:"tag_name"`
	URL        string  `json:"html_url"`
	Draft      bool    `json:"draft"`
	Prerelease bool    `json:"prerelease"`
	Assets     []Asset `json:"assets"`
}

type Checker struct {
	Client   *http.Client
	Download *http.Client
	BaseURL  string
	Repo     string
	Version  string
	OS       string
	Arch     string
}

func New(version string) Checker {
	return Checker{
		Client: &http.Client{Timeout: Timeout}, Download: &http.Client{Timeout: DownloadTimeout},
		BaseURL: API, Repo: Repo, Version: version, OS: runtime.GOOS, Arch: runtime.GOARCH,
	}
}

func (c Checker) Latest(ctx context.Context) (Release, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/repos/"+c.Repo+"/releases/latest", nil)
	if err != nil {
		return Release{}, fmt.Errorf("%w: %w", ErrBadRelease, err)
	}
	req.Header.Set("Accept", mimeJSON)
	req.Header.Set("User-Agent", "creds/"+c.Version)
	resp, err := c.client().Do(req)
	if err != nil {
		return Release{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if err := status(resp); err != nil {
		return Release{}, err
	}
	return decode(resp.Body)
}

func (c Checker) client() *http.Client {
	if c.Client != nil {
		return c.Client
	}
	return http.DefaultClient
}

func status(resp *http.Response) error {
	switch {
	case resp.StatusCode == http.StatusOK:
		return nil
	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		return ErrRateLimited
	}
	return fmt.Errorf("%w: status %d", ErrBadRelease, resp.StatusCode)
}

func decode(body io.Reader) (Release, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxBody+1))
	if err != nil {
		return Release{}, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	if len(data) > maxBody {
		return Release{}, fmt.Errorf("%w: response too large", ErrBadRelease)
	}
	var r Release
	if err := json.Unmarshal(data, &r); err != nil {
		return Release{}, fmt.Errorf("%w: %w", ErrBadRelease, err)
	}
	return r, r.check()
}

func (r Release) check() error {
	if r.Draft || r.Prerelease {
		return fmt.Errorf("%w: %s is not a full release", ErrBadRelease, r.Tag)
	}
	if _, ok := Parse(r.Tag); !ok {
		return fmt.Errorf("%w: bad tag %q", ErrBadRelease, r.Tag)
	}
	return nil
}
