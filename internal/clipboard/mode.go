package clipboard

import "fmt"

type Mode int

const (
	ModeAuto Mode = iota
	ModeNative
	ModeOSC52
)

func (m Mode) String() string {
	switch m {
	case ModeNative:
		return "native"
	case ModeOSC52:
		return "osc52"
	}
	return "auto"
}

func ParseMode(s string) (Mode, error) {
	for _, m := range []Mode{ModeAuto, ModeNative, ModeOSC52} {
		if s == m.String() {
			return m, nil
		}
	}
	return ModeAuto, fmt.Errorf("%w: %q", ErrBadMode, s)
}

func Resolve(m Mode, getenv func(string) string, goos string) Mode {
	if m != ModeAuto {
		return m
	}
	if overSSH(getenv) || getenv("HERDR_ENV") != "" && goos != "windows" {
		return ModeOSC52
	}
	return ModeNative
}

func overSSH(getenv func(string) string) bool {
	for _, k := range []string{"SSH_CONNECTION", "SSH_TTY", "SSH_CLIENT"} {
		if getenv(k) != "" {
			return true
		}
	}
	return false
}
