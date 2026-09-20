package cli

import (
	"errors"
	"strings"

	"github.com/elliot40404/creds/internal/search"
	"github.com/spf13/cobra"
)

var errNoPrompt = errors.New("prompt not allowed during completion")

type noPrompt struct{}

func (noPrompt) Password(string) (string, error)      { return "", errNoPrompt }
func (noPrompt) Confirm(string) (bool, error)         { return false, errNoPrompt }
func (noPrompt) Input(string, string) (string, error) { return "", errNoPrompt }
func (noPrompt) Select(string, []string) (int, error) { return 0, errNoPrompt }
func (noPrompt) Show(string, string) error            { return errNoPrompt }

func (e Env) allSummaries() ([]search.Summary, bool) {
	s, err := e.service(noPrompt{})
	if err != nil {
		return nil, false
	}
	s.Spawn = nil
	items, err := s.List()
	if err != nil {
		return nil, false
	}
	return items, true
}

func (e Env) completePaths(_ *cobra.Command, args []string, prefix string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return e.completeFrom(args, prefix, search.Paths, cobra.ShellCompDirectiveNoFileComp)
}

func (e Env) completeNewPath(_ *cobra.Command, args []string, prefix string) ([]cobra.Completion, cobra.ShellCompDirective) {
	folders := func(items []search.Summary) []string { return search.Prefixes(search.Paths(items)) }
	return e.completeFrom(args, prefix, folders, cobra.ShellCompDirectiveNoFileComp|cobra.ShellCompDirectiveNoSpace)
}

func (e Env) completeFrom(args []string, prefix string, pick func([]search.Summary) []string, dir cobra.ShellCompDirective) ([]cobra.Completion, cobra.ShellCompDirective) {
	none := cobra.ShellCompDirectiveNoFileComp
	if len(args) > 0 {
		return nil, none
	}
	items, ok := e.allSummaries()
	if !ok {
		return nil, none
	}
	var out []cobra.Completion
	for _, p := range pick(items) {
		if strings.HasPrefix(p, prefix) {
			out = append(out, p)
		}
	}
	return out, dir
}
