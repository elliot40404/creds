package buildinfo

import (
	"runtime"
	"runtime/debug"
	"strings"
)

var (
	version = ""
	commit  = ""
	date    = ""
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit,omitzero"`
	Date    string `json:"date,omitzero"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

func Get() Info {
	info := Info{Version: version, Commit: commit, Date: date, Go: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return info.orDev()
	}
	if info.Version == "" {
		info.Version = moduleVersion(bi)
	}
	dirty := false
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			if commit == "" {
				info.Commit = s.Value
			}
		case "vcs.time":
			if date == "" {
				info.Date = s.Value
			}
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if dirty && commit == "" && info.Commit != "" {
		info.Commit += "-dirty"
	}
	return info.orDev()
}

func (i Info) orDev() Info {
	if i.Version == "" {
		i.Version = "dev"
	}
	return i
}

func (i Info) String() string {
	parts := []string{"creds " + i.Version}
	if i.Commit != "" {
		parts = append(parts, "("+short(i.Commit)+")")
	}
	if i.Date != "" {
		parts = append(parts, i.Date)
	}
	parts = append(parts, i.Go, i.OS+"/"+i.Arch)
	return strings.Join(parts, " ")
}

func moduleVersion(bi *debug.BuildInfo) string {
	v := bi.Main.Version
	if v == "" || v == "(devel)" || strings.HasPrefix(v, "v0.0.0-") {
		return ""
	}
	return v
}

func short(commit string) string {
	base, dirty, _ := strings.Cut(commit, "-")
	if len(base) > 12 {
		base = base[:12]
	}
	if dirty != "" {
		return base + "-" + dirty
	}
	return base
}
