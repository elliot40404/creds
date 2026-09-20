package clipboard

import (
	"encoding/base64"
	"errors"
	"io"
	"strings"
)

const MaxOSC52 = 74994

var ErrTooLarge = errors.New("value too large for OSC52")

const (
	esc       = "\x1b"
	oscStart  = esc + "]52;c;"
	oscEnd    = "\a"
	tmuxStart = esc + "Ptmux;"
	tmuxEnd   = esc + `\`
)

func Sequence(value string, tmux bool) ([]byte, error) {
	if len(value) > MaxOSC52 {
		return nil, ErrTooLarge
	}
	seq := oscStart + base64.StdEncoding.EncodeToString([]byte(value)) + oscEnd
	if tmux {
		seq = tmuxStart + strings.ReplaceAll(seq, esc, esc+esc) + tmuxEnd
	}
	return []byte(seq), nil
}

type Terminal struct {
	W    io.Writer
	Tmux bool
}

func (t Terminal) Write(value string) error {
	seq, err := Sequence(value, t.Tmux)
	if err != nil {
		return err
	}
	_, err = t.W.Write(seq)
	return err
}
