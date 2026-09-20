package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/elliot40404/creds/internal/fsutil"
)

var clipboardTools = []string{"wl-copy", "xclip", "xsel"}

func (u *Setup) Preflight() []Check {
	return []Check{u.checkGit(), u.checkClipboard(), u.checkHome(), u.checkHomeAccess(), u.checkHomeContents()}
}

func (u *Setup) look(file string) (string, error) {
	if u.Look == nil {
		return "", ErrNoLookup
	}
	return u.Look(file)
}

func (u *Setup) goos() string {
	if u.OS != "" {
		return u.OS
	}
	return runtime.GOOS
}

func (u *Setup) checkGit() Check {
	c := Check{Name: "git"}
	path, err := u.look("git")
	if err != nil {
		c.Message = "git not found on PATH"
		c.Fix = "install git, sync needs it, the vault itself works without it"
		return c
	}
	c.OK, c.Message = true, "git at "+path
	return c
}

func (u *Setup) checkClipboard() Check {
	c := Check{Name: "clipboard"}
	if u.goos() != "linux" {
		c.OK, c.Message = true, "clipboard handled by the system"
		return c
	}
	for _, tool := range clipboardTools {
		if _, err := u.look(tool); err == nil {
			c.OK, c.Message = true, tool+" found"
			return c
		}
	}
	c.Message = "no clipboard tool found"
	c.Fix = fmt.Sprintf("install one of %v, or copy over ssh with creds copy --osc52", clipboardTools)
	return c
}

func (u *Setup) checkHome() Check {
	home := u.Service.Paths.Home
	c := Check{Name: "home"}
	if err := u.writable(home); err != nil {
		c.Message = fmt.Sprintf("%s is not writable: %v", home, err)
		c.Fix = "fix the permissions, or set CREDS_HOME to a writable directory"
		return c
	}
	c.OK, c.Message = true, home+" is writable"
	return c
}

func (u *Setup) checkHomeAccess() Check {
	home := u.Service.Paths.Home
	c := Check{Name: "home access"}
	msg, err := fsutil.CheckPerms(home)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		c.OK, c.Message = true, home+" does not exist yet"
	case err != nil:
		c.Message = fmt.Sprintf("could not check access to %s: %v", home, err)
		c.Fix = "check the directory permissions, or set CREDS_HOME elsewhere"
	case msg != "":
		c.Message = msg
		c.Fix = "another account can reach your vault, fix this before you continue"
	default:
		c.OK, c.Message = true, "only you can reach "+home
	}
	return c
}

func (u *Setup) checkHomeContents() Check {
	c := Check{Name: "home contents"}
	has, err := u.Service.hasVault()
	if err != nil {
		c.Message = fmt.Sprintf("could not read %s: %v", u.Service.Paths.Home, err)
		return c
	}
	stray := u.Service.strayFiles()
	switch {
	case has || len(stray) == 0:
		c.OK, c.Message = true, "nothing unexpected in "+u.Service.Paths.Home
	default:
		c.Message = fmt.Sprintf("%s holds %s but no vault", u.Service.Paths.Home, strings.Join(stray, ", "))
		c.Fix = "setup will ask before it keeps them, delete them if you did not write them." + u.Service.configSetNote(stray)
	}
	return c
}

func (u *Setup) writable(home string) error {
	if err := fsutil.EnsureDir(home); err != nil {
		return err
	}
	path := filepath.Join(home, ".writable")
	if err := fsutil.WriteFileAtomic(path, nil); err != nil {
		return err
	}
	return os.Remove(path)
}
