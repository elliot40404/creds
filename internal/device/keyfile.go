package device

import (
	"errors"
	"fmt"
	"strings"
)

const maxKeySize = 1 << 16

var ErrBadKey = errors.New("device: not an age plugin identity file")

var recipientLabels = []string{"recipient:", "public key:"}

func ParseKey(text string) (identity, recipient string, err error) {
	if len(text) > maxKeySize {
		return "", "", fmt.Errorf("%w: too large", ErrBadKey)
	}
	for line := range strings.Lines(text) {
		line = strings.TrimSpace(line)
		switch {
		case line == "":
		case strings.HasPrefix(line, "#"):
			if r, ok := commentRecipient(line); ok && recipient == "" {
				recipient = r
			}
		case strings.HasPrefix(line, "AGE-PLUGIN-") && identity == "":
			identity = line
		default:
			return "", "", fmt.Errorf("%w: want one AGE-PLUGIN- line and comments", ErrBadKey)
		}
	}
	if identity == "" {
		return "", "", fmt.Errorf("%w: no AGE-PLUGIN- line", ErrBadKey)
	}
	return identity, recipient, nil
}

func commentRecipient(line string) (string, bool) {
	body := strings.TrimSpace(strings.TrimPrefix(line, "#"))
	for _, label := range recipientLabels {
		if len(body) < len(label) || !strings.EqualFold(body[:len(label)], label) {
			continue
		}
		fields := strings.Fields(body[len(label):])
		if len(fields) == 1 && strings.HasPrefix(fields[0], "age1") {
			return fields[0], true
		}
	}
	return "", false
}
