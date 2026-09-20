package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/spf13/cobra"
)

type hintRule struct {
	target  error
	needPos bool
	hint    func(cmd *cobra.Command, pos, args []string) string
}

var typedHints = []hintRule{
	{vault.ErrNotFound, true, func(_ *cobra.Command, pos, _ []string) string {
		return "run " + shellLine([]string{"creds", "search", pos[0]})
	}},
	{render.ErrUnknownFormat, true, func(_ *cobra.Command, pos, _ []string) string {
		return "run " + shellLine([]string{"creds", "get", pos[0], "--formats"})
	}},
	{errNeedYes, false, yesHint},
}

func fillHint(cmd *cobra.Command, args []string, fe *fail.Error) *fail.Error {
	out := *fe
	out.Hint = strings.ReplaceAll(out.Hint, fail.ThisCommand, cmd.CommandPath())
	pos := entryArgs(cmd)
	for _, r := range typedHints {
		if errors.Is(fe, r.target) && (len(pos) > 0 || !r.needPos) {
			out.Hint = r.hint(cmd, pos, args)
			break
		}
	}
	if cerr, ok := errors.AsType[*app.ConflictError](fe); ok && len(cerr.Paths) > 0 {
		out.Hint = conflictHint(cerr.Paths)
	}
	return &out
}

func conflictHint(paths []string) string {
	hint := "run " + shellLine([]string{"creds", "resolve", paths[0]}) + " --mine|--theirs"
	if len(paths) > 1 {
		hint += ", then the same for the other conflicts"
	}
	return hint
}

func entryArgs(cmd *cobra.Command) []string {
	pos := cmd.Flags().Args()
	if at := cmd.ArgsLenAtDash(); at >= 0 {
		pos = pos[:at]
	}
	usage := strings.Fields(cmd.Use)
	if len(pos) == 0 || len(usage) < 2 || strings.Trim(usage[1], "<>[]") != "path" {
		return nil
	}
	return pos
}

func yesHint(cmd *cobra.Command, _, args []string) string {
	if cmd.Flags().Lookup("yes") == nil {
		return "run " + shellLine(append([]string{"creds"}, args...)) + " in a terminal and answer the question"
	}
	return "run " + shellLine(withYes(args))
}

func withYes(args []string) []string {
	out := append([]string{"creds"}, args...)
	at := slices.Index(out, "--")
	if at < 0 {
		return append(out, "--yes")
	}
	return slices.Insert(out, at, "--yes")
}
