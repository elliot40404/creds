package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/render"
	"github.com/spf13/cobra"
)

const (
	flagInteractive = "interactive"
	flagJSON        = "json"
)

var (
	errNeedTTY    = errors.New("-i needs an interactive terminal")
	errNoPlan     = errors.New("command has no interactive mode")
	errNoCommands = errors.New("no command to pick")
)

type interview struct {
	env   Env
	p     app.Prompter
	cmd   *cobra.Command
	args  []string
	tail  []string
	dash  bool
	conn  []string
	stdin []string
	svc   *app.Service
	code  int
	ran   bool
}

func (e Env) interactiveCmd(args []string) (*cobra.Command, bool) {
	cmd, rest, err := NewRoot(e).Find(args)
	if err != nil || cmd.ParseFlags(rest) != nil {
		return nil, false
	}
	on, err := cmd.Flags().GetBool(flagInteractive)
	return cmd, err == nil && on
}

func (e Env) interview(cmd *cobra.Command, args []string) int {
	iv := &interview{env: e, p: e.Prompter}
	iv.load(cmd)
	if err := iv.start(); err != nil {
		return e.report(cmd, args, runError{err})
	}
	return iv.code
}

func (iv *interview) load(cmd *cobra.Command) {
	iv.cmd = cmd
	fl := cmd.Flags()
	iv.args, iv.tail, iv.dash = fl.Args(), nil, false
	if at := cmd.ArgsLenAtDash(); at >= 0 {
		iv.args, iv.tail, iv.dash = iv.args[:at], iv.args[at:], true
	}
}

func (iv *interview) start() error {
	if !isTTY(iv.p) {
		return errNeedTTY
	}
	if err := iv.follow(); err != nil {
		return err
	}
	if !iv.ran {
		iv.exec()
	}
	return nil
}

func (iv *interview) follow() error {
	pl, ok := plans()[planKey(iv.cmd)]
	if !ok {
		return errNoPlan
	}
	if pl.run == nil {
		return nil
	}
	if pl.vault {
		if err := iv.unlock(); err != nil {
			return err
		}
	}
	return pl.run(iv)
}

func (iv *interview) switchTo(words []string, args ...string) error {
	cmd, _, err := iv.cmd.Root().Find(words)
	if err != nil {
		return err
	}
	if err := cmd.ParseFlags(nil); err != nil {
		return err
	}
	iv.load(cmd)
	iv.args = args
	return iv.follow()
}

func (iv *interview) pickCommand() error {
	var names []string
	for _, c := range iv.cmd.Commands() {
		if c.IsAvailableCommand() && c.Name() != "help" && c.Name() != "completion" {
			names = append(names, c.Name())
		}
	}
	if len(names) == 0 {
		return errNoCommands
	}
	i, err := iv.p.Select("Action", names)
	if err != nil {
		return err
	}
	return iv.switchTo(append(cmdWords(iv.cmd), names[i]))
}

func (iv *interview) exec() {
	iv.run(iv.argv(), append(slices.Clone(iv.conn), iv.stdin...))
}

func (iv *interview) run(argv, stdin []string) {
	env := iv.env
	if iv.ran {
		env.Spawn = nil
	}
	if len(stdin) > 0 {
		env.In = strings.NewReader(strings.Join(stdin, "\n") + "\n")
	}
	iv.code = env.execute(argv)
	iv.ran = true
	line := shellLine(append([]string{"creds"}, argv...))
	if len(stdin) > 0 {
		line += "   (secret values on stdin, one per line)"
	}
	env.note("command: %s", line)
}

func (iv *interview) argv() []string {
	var flags []string
	pl := plans()[planKey(iv.cmd)]
	names := append(append([]string{flagJSON}, pl.asks...), pl.skip...)
	slices.Sort(names)
	for _, name := range slices.Compact(names) {
		if iv.changed(name) {
			flags = append(flags, iv.flagTokens(name)...)
		}
	}
	out := cmdWords(iv.cmd)
	switch {
	case iv.dash:
		return slices.Concat(out, iv.args, flags, []string{"--"}, iv.tail)
	case slices.ContainsFunc(iv.args, func(a string) bool { return strings.HasPrefix(a, "-") }):
		return slices.Concat(out, flags, []string{"--"}, iv.args)
	}
	return slices.Concat(out, iv.args, flags)
}

func (iv *interview) flagTokens(name string) []string {
	f := iv.cmd.Flag(name)
	flag := "--" + name
	switch f.Value.Type() {
	case "bool":
		if f.Value.String() == "true" {
			return []string{flag}
		}
		return []string{flag + "=false"}
	case "stringArray":
		var out []string
		for _, v := range iv.list(name) {
			out = append(out, flag, v)
		}
		return out
	}
	return []string{flag, f.Value.String()}
}

func cmdWords(c *cobra.Command) []string {
	words := strings.Fields(c.CommandPath())
	return slices.Clone(words[1:])
}

func planKey(c *cobra.Command) string {
	return strings.Join(cmdWords(c), " ")
}

func isTTY(p app.Prompter) bool {
	t, ok := p.(interface{ Interactive() bool })
	return ok && t.Interactive()
}

func shellLine(argv []string) string {
	return quoteLine(render.DefaultShell(), argv)
}

func quoteLine(sh render.Shell, argv []string) string {
	out := make([]string, len(argv))
	for i, a := range argv {
		out[i] = sh.Quote(a)
	}
	return strings.Join(out, " ")
}
