package update

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	tempDownload = ".creds-download-"
	tempNew      = ".creds-new-"
	staleAge     = time.Hour
)

func RemoveStale(dir string, now time.Time) {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if isStale(e, now) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}

func isStale(e os.DirEntry, now time.Time) bool {
	name := e.Name()
	if !e.Type().IsRegular() || !strings.HasPrefix(name, tempDownload) && !strings.HasPrefix(name, tempNew) {
		return false
	}
	info, err := e.Info()
	return err == nil && now.Sub(info.ModTime()) > staleAge
}
