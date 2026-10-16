package app

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const sshHostDir = "config.d/"

var (
	ErrSSHHost   = errors.New("bad ssh host")
	ErrSSHExists = errors.New("ssh file already exists")
)

var sshNamePart = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type SSHHost struct {
	Name string
	Host string
	User string
	Port int
	Key  string
}

func (h SSHHost) Rel() string {
	return sshHostDir + h.Name
}

func (h SSHHost) Alias() string {
	return path.Base(h.Name)
}

func (h SSHHost) IncludeLine() string {
	return "Include " + sshHostDir + strings.Repeat("*/", strings.Count(h.Name, "/")) + "*"
}

func (h SSHHost) check() error {
	if err := checkHostName(h.Name); err != nil {
		return err
	}
	switch {
	case h.Host == "":
		return fmt.Errorf("%w: pass --host with a host name or IP", ErrSSHHost)
	case h.Port < 0 || h.Port > 65535:
		return fmt.Errorf("%w: port %d is not 1-65535", ErrSSHHost, h.Port)
	}
	for _, v := range [][2]string{{"host", h.Host}, {"user", h.User}, {"key", h.Key}} {
		if !sshWord(v[1]) {
			return fmt.Errorf("%w: %s %q has a space, quote, # or control character", ErrSSHHost, v[0], v[1])
		}
	}
	return nil
}

func checkHostName(name string) error {
	parts := strings.Split(name, "/")
	if len(parts) > 2 {
		return fmt.Errorf("%w: %q has more than one /, use group/name", ErrSSHHost, name)
	}
	for _, p := range parts {
		if !sshNamePart.MatchString(p) {
			return fmt.Errorf("%w: %q, use letters, digits, . _ - and one / like group/name", ErrSSHHost, name)
		}
	}
	return nil
}

func sshWord(s string) bool {
	return !strings.ContainsFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || strings.ContainsRune(`"'#\`, r)
	})
}

func (h SSHHost) config() []byte {
	var b strings.Builder
	b.WriteString("Host " + h.Alias() + "\n  HostName " + h.Host + "\n")
	if h.User != "" {
		b.WriteString("  User " + h.User + "\n")
	}
	if h.Port != 0 && h.Port != 22 {
		b.WriteString("  Port " + strconv.Itoa(h.Port) + "\n")
	}
	if h.Key != "" {
		b.WriteString("  IdentityFile ~/.ssh/" + h.Key + "\n  IdentitiesOnly yes\n")
	}
	return []byte(b.String())
}
