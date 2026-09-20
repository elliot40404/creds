//go:build !windows

package fail

var osKnown map[string]error

var osOnly = []string{"clipboard.ErrOSC52ClearUnsupported"}
