package fail

import "github.com/elliot40404/creds/internal/clipboard"

var osRules = []Rule{
	{Target: clipboard.ErrOSC52ClearUnsupported, Code: General},
}
