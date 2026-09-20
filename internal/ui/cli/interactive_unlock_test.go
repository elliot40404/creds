package cli

import (
	"slices"
	"testing"
)

const masterPrompt = "Master password"

func TestInteractiveUnlocksBeforeQuestions(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	cmds := []string{"add", "edit", "get", "copy", "rm", "list", "search", "env", "run", "import-env", "import", "resolve"}
	for _, name := range cmds {
		h.expire()
		f := &fake{passwords: []string{mainWord}}
		h.run(f, name, "-i")
		if len(f.prompts) == 0 || f.prompts[0] != masterPrompt {
			t.Fatalf("%s prompts %q", name, f.prompts)
		}
	}
}

func TestInteractiveWrongPasswordAsksNothingElse(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.expire()
	f := &fake{passwords: []string{"no", "no", "no"}, inputs: []string{"web/x"}}
	h.fail(f, "add", "-i")
	if len(f.inputs) != 1 || slices.ContainsFunc(f.prompts, func(p string) bool { return p != masterPrompt }) {
		t.Fatalf("prompts %q", f.prompts)
	}
}

func TestInteractivePlainExportUnlocksAfterKind(t *testing.T) {
	t.Parallel()
	h := seeded(t)
	h.expire()
	f := &fake{picks: []string{"plain"}, passwords: []string{mainWord}}
	h.run(f, "export", "-i")
	if len(f.prompts) < 3 || f.prompts[0] != "Export" || f.prompts[1] != masterPrompt {
		t.Fatalf("prompts %q", f.prompts)
	}
}
