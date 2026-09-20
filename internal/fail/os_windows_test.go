package fail

import "github.com/elliot40404/creds/internal/clipboard"

var osKnown = map[string]error{
	"clipboard.ErrOSC52ClearUnsupported": clipboard.ErrOSC52ClearUnsupported,
}

var osOnly []string
