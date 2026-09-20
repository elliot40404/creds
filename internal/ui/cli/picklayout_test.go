package cli

import (
	"testing"

	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/ui/tui"
	"github.com/spf13/cobra"
)

// layoutPicker records the layout the picker was opened with.
func layoutPicker(got *tui.Layout) picker {
	return picker{
		interactive: func() bool { return true },
		open: func(opts tui.Options) error {
			*got = opts.Layout
			return nil
		},
	}
}

func openedLayout(t *testing.T, h *harness, f *fake, args ...string) tui.Layout {
	t.Helper()
	var got tui.Layout
	env, _, errb := h.env(f)
	root := &cobra.Command{Use: "creds", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(env.Out)
	root.SetErr(env.Err)
	root.AddCommand(pickCmdWith(env, root, layoutPicker(&got)))
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("%v: %v, stderr %q", args, err, errb.String())
	}
	return got
}

func TestPickDefaultsToFullscreen(t *testing.T) {
	h := seeded(t)
	f := &fake{}
	got := openedLayout(t, h, f, "pick")
	if want := (tui.Layout{AltScreen: true, Height: config.DefaultHeight}); got != want {
		t.Fatalf("layout %#v, want %#v", got, want)
	}
}

func TestPickInlineFlag(t *testing.T) {
	h := seeded(t)
	got := openedLayout(t, h, &fake{}, "pick", "--inline")
	if !got.Inline || got.Height != config.DefaultHeight {
		t.Fatalf("layout %#v", got)
	}
}

func TestPickHeightFlagClamps(t *testing.T) {
	h := seeded(t)
	if got := openedLayout(t, h, &fake{}, "pick", "--inline", "--height", "999"); got.Height != config.MaxHeight {
		t.Fatalf("height %d, want %d", got.Height, config.MaxHeight)
	}
	if got := openedLayout(t, h, &fake{}, "pick", "--inline", "--height", "1"); got.Height != config.MinHeight {
		t.Fatalf("height %d, want %d", got.Height, config.MinHeight)
	}
}

func TestPickAltScreenFlag(t *testing.T) {
	h := seeded(t)
	if got := openedLayout(t, h, &fake{}, "pick", "--alt-screen=false"); got.AltScreen {
		t.Fatal("alt screen should be off")
	}
	if got := openedLayout(t, h, &fake{}, "pick", "--inline", "--alt-screen=false"); got.AltScreen || !got.Inline {
		t.Fatalf("layout %#v", got)
	}
}

func TestPickReadsConfig(t *testing.T) {
	h := seeded(t)
	h.ok(&fake{}, "config", "set", "ui.mode", "inline")
	h.ok(&fake{}, "config", "set", "ui.height", "22")
	h.ok(&fake{}, "config", "set", "ui.altscreen", "false")
	got := openedLayout(t, h, &fake{}, "pick")
	if want := (tui.Layout{Inline: true, Height: 22}); got != want {
		t.Fatalf("layout %#v, want %#v", got, want)
	}
}

func TestPickFullscreenFlagBeatsConfig(t *testing.T) {
	h := seeded(t)
	h.ok(&fake{}, "config", "set", "ui.mode", "inline")
	if got := openedLayout(t, h, &fake{}, "pick", "--fullscreen"); got.Inline {
		t.Fatalf("layout %#v, want fullscreen", got)
	}
}

func TestPickInlineAndFullscreenConflict(t *testing.T) {
	h := seeded(t)
	if r := h.run(&fake{}, "pick", "--inline", "--fullscreen"); r.code == 0 {
		t.Fatal("both flags should be rejected")
	}
}
