package fail

import "github.com/elliot40404/creds/internal/gitsync"

func SyncText(raw string) (msg, hint string) {
	if cause := gitsync.Cause(raw); cause != nil {
		e := Classify(cause)
		return e.Msg, e.Hint
	}
	return gitsync.Detail(raw), ""
}
