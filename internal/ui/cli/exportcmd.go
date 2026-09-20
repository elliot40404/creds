package cli

import (
	"os"
	"path/filepath"

	"github.com/elliot40404/creds/internal/app"
	"github.com/spf13/cobra"
)

func exportCommands(env Env) []*cobra.Command {
	return []*cobra.Command{exportCmd(env), importCmd(env)}
}

func exportCmd(env Env) *cobra.Command {
	var out, format, confirm string
	var encrypted, yes bool
	c := &cobra.Command{
		Use:   "export --plain|--encrypted -o <file>",
		Short: "Export the vault to a file",
		Args:  cobra.NoArgs,
		RunE: env.run(func(s *app.Service, _ []string) error {
			export := func() (int, error) { return exportService(s, confirm).ExportPlain(out, format, yes) }
			what := "entries"
			if encrypted {
				export = func() (int, error) { return s.ExportEncrypted(out, yes) }
				what = "files"
			}
			n, err := export()
			if err != nil {
				return err
			}
			env.note("exported %d %s to %s", n, what, out)
			return nil
		}),
	}
	c.Flags().Bool("plain", false, "write secrets unencrypted")
	c.Flags().BoolVar(&encrypted, "encrypted", false, "write a tar of the encrypted vault files")
	c.Flags().StringVar(&format, "format", app.FormatJSON, "plain format: json, or csv for a partial view that cannot be imported")
	c.Flags().StringVarP(&out, "output", "o", "", "file to write")
	c.Flags().BoolVarP(&yes, "yes", "y", false, "overwrite an existing file")
	c.Flags().StringVar(&confirm, "confirm-plaintext", "", "the plaintext phrase, for use without a terminal")
	c.MarkFlagsOneRequired("plain", "encrypted")
	c.MarkFlagsMutuallyExclusive("plain", "encrypted")
	c.MarkFlagsMutuallyExclusive("encrypted", "format")
	c.MarkFlagsMutuallyExclusive("encrypted", "confirm-plaintext")
	_ = c.MarkFlagRequired("output")
	return c
}

func importCmd(env Env) *cobra.Command {
	var overwrite, yes bool
	c := &cobra.Command{
		Use:   "import <file.json>",
		Short: "Import entries from a json export",
		Args:  cobra.ExactArgs(1),
		RunE: env.run(func(s *app.Service, args []string) error {
			f, err := os.Open(filepath.Clean(args[0]))
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()
			if overwrite {
				if err := env.confirm(yes, "Replace entries that have the same path?"); err != nil {
					return err
				}
			}
			res, err := s.Import(f, overwrite)
			if err != nil {
				return err
			}
			for _, p := range res.Skipped {
				env.note("skipped %s: path exists, use --overwrite", p)
			}
			env.note("imported %d, replaced %d, skipped %d", len(res.Added), len(res.Replaced), len(res.Skipped))
			return nil
		}),
	}
	c.Flags().BoolVar(&overwrite, "overwrite", false, "replace entries with the same path")
	yesFlag(c, &yes)
	return c
}

type phrase struct {
	app.Prompter
	text string
}

func (p phrase) Input(string, string) (string, error) { return p.text, nil }

func exportService(s *app.Service, confirm string) *app.Service {
	if confirm != "" {
		s.Prompter = phrase{s.Prompter, confirm}
	}
	return s
}
