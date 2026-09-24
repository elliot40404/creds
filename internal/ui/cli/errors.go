package cli

import (
	"errors"
	"strings"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/ui/tui"
	"github.com/spf13/cobra"
)

const (
	hintFlags       = "pass flags instead, run creds help agents"
	hintSecretStdin = "pass --secret-field <name> or --conn - and write one value per line to stdin"
	hintConn        = "pass --engine postgres|redis|mongo --conn - and write the connection string to stdin"
)

var hintType = "pass --type " + strings.Join(app.TypeNames(), "|")

var cliRules = []fail.Rule{
	{Target: ErrNotTerminal, Code: fail.Locked, Hint: "run creds unlock in a terminal first"},
	{Target: errNeedYes, Code: fail.Usage, Hint: "rerun the creds command with --yes"},
	{Target: errNeedFlags, Code: fail.Usage, Hint: hintFlags},
	{Target: errTrustSource, Code: fail.Usage, Hint: "run creds device trust --touchid or creds device trust --identity <file>"},
	{Target: ErrNoInput, Code: fail.Usage, Hint: hintFlags},
	{Target: ErrNeedDash, Code: fail.Usage, Hint: "run creds run [path] -- <command> [args...]"},
	{Target: errNoProgram, Code: fail.NotFound, Hint: "check the program name, or install it and add it to PATH"},
	{Target: ErrBadChoice, Code: fail.Usage, Hint: "type the number of one of the listed options"},
	{Target: ErrUnknownType, Code: fail.Usage, Hint: hintType},
	{Target: ErrDuplicateField, Code: fail.Usage, Hint: "pick another field name"},
	{Target: errSide, Code: fail.Usage, Hint: "run creds resolve <path> --mine or --theirs"},
	{Target: errSecretInArgs, Code: fail.Usage, Hint: hintSecretStdin},
	{Target: errShortStdin, Code: fail.Usage, Hint: hintSecretStdin},
	{Target: errSecretBuiltin, Code: fail.Usage, Hint: "pass --username, --host, --url or --notes for those, or pick another secret field name"},
	{Target: errFieldName, Code: fail.Usage, Hint: "type a field name without =, like api_key"},
	{Target: errFieldFormat, Code: fail.Usage, Hint: "pass --field key=value"},
	{Target: errNeedType, Code: fail.Usage, Hint: hintType},
	{Target: errUnknownSort, Code: fail.Usage, Hint: "pass --sort " + strings.Join(search.Sorts(), "|")},
	{Target: errNeedEngine, Code: fail.Usage, Hint: hintConn},
	{Target: errEnvFormat, Code: fail.Usage, Hint: "pass --format dotenv or --format sh"},
	{Target: errEngineAlone, Code: fail.Usage, Hint: hintConn},
	{Target: errNotDatabase, Code: fail.Usage, Hint: "run creds get <path> to check the entry type"},
	{Target: errNeedTTY, Code: fail.Usage, Hint: "drop -i and pass flags, run creds help agents"},
	{Target: errNoMatch, Code: fail.Usage, Hint: "run creds list to see entries"},
	{Target: errNeedPath, Code: fail.Usage, Hint: "type a path like web/github"},
	{Target: errNoConflicts, Code: fail.Usage, Hint: "run creds status"},
	{Target: tui.ErrRemoteNoVault, Code: fail.Usage, Hint: "run creds setup and create a new vault, or check the url"},
	{Target: tui.ErrHomeBroken, Code: fail.General, Hint: "fix the creds home as shown above, then run creds setup again"},
	{Target: tui.ErrNeedPassword, Code: fail.Locked, Hint: "run creds unlock"},
}

type runError struct{ err error }

func (e runError) Error() string { return e.err.Error() }

func (e runError) Unwrap() error { return e.err }

func markRun(c *cobra.Command) {
	if fn := c.RunE; fn != nil {
		c.RunE = func(cmd *cobra.Command, args []string) error {
			if err := fn(cmd, args); err != nil {
				return runError{err}
			}
			return nil
		}
	}
	for _, sub := range c.Commands() {
		markRun(sub)
	}
}

func classify(cmd *cobra.Command, err error) *fail.Error {
	if _, ok := errors.AsType[runError](err); ok {
		return fail.Classify(err, cliRules...)
	}
	if e, ok := errors.AsType[*fail.Error](err); ok {
		return e
	}
	e := fail.Classify(err, cliRules...)
	if e.Code == fail.General {
		e.Code = fail.Usage
		e.Hint = "run " + cmd.CommandPath() + " --help"
	}
	return e
}
