package envfile

import (
	"slices"
	"strings"
)

const riskyNote = "  (risky: changes how programs run or where they connect)"

var riskyKeys = []string{"BASH_ENV", "ENV", "NODE_OPTIONS", "GIT_SSH_COMMAND", "GIT_ASKPASS", "PATH", "PYTHONPATH", "PERL5OPT", "RUBYOPT"}

func risky(key string) bool {
	k := strings.ToUpper(key)
	return slices.Contains(riskyKeys, k) || strings.HasSuffix(k, "_PROXY") ||
		strings.HasPrefix(k, "LD_") || strings.HasPrefix(k, "DYLD_")
}
