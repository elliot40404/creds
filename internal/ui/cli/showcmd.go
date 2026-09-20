package cli

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/fail"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/safetext"
	"github.com/elliot40404/creds/internal/search"
	"github.com/elliot40404/creds/internal/vault"
	"github.com/spf13/cobra"
)

const mask = "********"

func getCmd(env Env) *cobra.Command {
	var show, formats bool
	var field, as, shell string
	c := &cobra.Command{
		Use:               "get <path>",
		Short:             "Show an entry",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: env.completePaths,
		RunE: env.run(func(s *app.Service, args []string) error {
			sh, err := renderShell(shell, as)
			if err != nil {
				return err
			}
			switch {
			case formats:
				return env.printFormats(s, args[0])
			case as != "":
				v, err := s.Render(args[0], as, sh)
				return env.printValue(jsonValue{Path: args[0], Format: as, Value: v}, err)
			}
			e, err := s.Get(args[0])
			if err != nil {
				return err
			}
			if field != "" {
				v, err := app.Lookup(e, field)
				return env.printValue(jsonValue{Path: e.Path, Field: field, Value: v}, err)
			}
			if env.jsonMode() {
				return env.writeJSON(toJSONEntry(e, show))
			}
			return writeEntry(env.Out, e, show)
		}),
	}
	c.Flags().BoolVar(&show, "show", false, "reveal secret values")
	c.Flags().StringVar(&field, "field", "", "print only this raw value")
	c.Flags().StringVar(&as, "as", "", "print a database entry in this format")
	c.Flags().BoolVar(&formats, "formats", false, "list formats for a database entry")
	c.Flags().StringVar(&shell, "shell", "", "quote for this shell instead of render.shell, needs --as")
	c.MarkFlagsMutuallyExclusive("field", "as", "formats")
	return c
}

func (e Env) printValue(v jsonValue, err error) error {
	switch {
	case err != nil:
		return err
	case e.jsonMode():
		return e.writeJSON(v)
	}
	return e.writeRaw(v.Value + "\n")
}

func (e Env) printFormats(s *app.Service, path string) error {
	names, err := s.Formats(path)
	switch {
	case err != nil:
		return err
	case e.jsonMode():
		return e.writeJSON(jsonFormats{Path: path, Formats: nonNil(names)})
	}
	for _, n := range names {
		if _, err := fmt.Fprintln(e.Out, safetext.Line(n)); err != nil {
			return err
		}
	}
	return nil
}

func writeEntry(w io.Writer, e vault.Entry, show bool) error {
	rows := [][2]string{{"path", e.Path}, {"type", string(e.Type)}}
	for _, k := range []string{app.FieldUsername, app.FieldHost, app.FieldURL, app.FieldNotes} {
		if v := *app.BuiltinField(&e, k); v != "" {
			rows = append(rows, [2]string{k, v})
		}
	}
	for _, f := range e.Fields {
		v := f.Value
		if f.Secret && !show {
			v = mask
		}
		rows = append(rows, [2]string{f.Name, v})
	}
	for _, k := range slices.Sorted(maps.Keys(e.Params)) {
		v := e.Params[k]
		if render.SecretParam(k) && !show {
			v = mask
		}
		rows = append(rows, [2]string{k, v})
	}
	rows = append(rows, [2]string{"created", e.Created.Local().Format(time.RFC3339)})
	rows = append(rows, [2]string{"updated", e.Updated.Local().Format(time.RFC3339)})
	if e.Machine != "" {
		rows = append(rows, [2]string{"machine", e.Machine})
	}
	if n := len(e.History); n > 0 {
		rows = append(rows, [2]string{"versions", strconv.Itoa(n) + " kept, newest " + e.History[0].At.Local().Format(time.RFC3339)})
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, r := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%s\n", safetext.Line(r[0]), safetext.Line(r[1])); err != nil {
			return err
		}
	}
	return tw.Flush()
}

var errUnknownSort = errors.New("not a sort order")

func sortFlag(c *cobra.Command, order *string) {
	c.Flags().StringVar(order, "sort", string(search.SortPath),
		"order the list: "+strings.Join(search.Sorts(), "|"))
}

func parseSort(order string) (search.Sort, error) {
	s := search.Sort(order)
	if !s.Valid() {
		return "", fmt.Errorf("%w: %q", errUnknownSort, order)
	}
	return s, nil
}

func listCmd(env Env) *cobra.Command {
	var order string
	c := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List entries",
		Args:    cobra.NoArgs,
		RunE: env.run(func(s *app.Service, _ []string) error {
			by, err := parseSort(order)
			if err != nil {
				return err
			}
			items, err := s.Search("", by)
			if err != nil {
				return err
			}
			return env.summaries(items)
		}),
	}
	sortFlag(c, &order)
	return c
}

func searchCmd(env Env) *cobra.Command {
	var order string
	c := &cobra.Command{
		Use:   "search <query>",
		Short: "Fuzzy search entries",
		Args:  cobra.MinimumNArgs(1),
		RunE: env.run(func(s *app.Service, args []string) error {
			by, err := parseSort(order)
			if err != nil {
				return err
			}
			items, err := s.Search(strings.Join(args, " "), by)
			if err != nil {
				return err
			}
			return env.summaries(items)
		}),
	}
	sortFlag(c, &order)
	return c
}

func (e Env) summaries(items []search.Summary) error {
	if e.jsonMode() {
		return e.writeJSON(toJSONSummaries(items))
	}
	tw := tabwriter.NewWriter(e.Out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "PATH\tTYPE\tHOST\tUSERNAME"); err != nil {
		return err
	}
	for _, it := range items {
		cols := []string{it.Path, string(it.Type), it.Host, it.Username}
		for i, c := range cols {
			cols[i] = safetext.Line(c)
		}
		if _, err := fmt.Fprintln(tw, strings.Join(cols, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

func (e Env) writeRaw(s string) error {
	_, err := io.WriteString(e.Out, rawText(s, isTerminal(e.Out)))
	return err
}

func rawText(s string, tty bool) string {
	if tty {
		return safetext.Text(s)
	}
	return s
}

var errShellNeedsAs = errors.New("--shell only applies to --as")

func renderShell(shell, as string) (render.Shell, error) {
	if shell == "" {
		return "", nil
	}
	if as == "" {
		return "", &fail.Error{
			Msg:  errShellNeedsAs.Error(),
			Hint: "drop --shell, or pick a format with --as, for example --as psql",
			Code: fail.Usage,
			Err:  errShellNeedsAs,
		}
	}
	return render.ParseShell(shell)
}
