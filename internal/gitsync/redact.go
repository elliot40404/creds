package gitsync

import (
	"strings"

	"github.com/elliot40404/creds/internal/safetext"
)

func redact(text string) string {
	for f := range strings.FieldsSeq(text) {
		if strings.Contains(f, "://") {
			text = strings.ReplaceAll(text, f, safetext.Remote(f))
		}
	}
	return text
}
