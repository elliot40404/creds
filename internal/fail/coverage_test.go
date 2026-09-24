package fail

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/elliot40404/creds/internal/anchor"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/clipboard"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/device"
	"github.com/elliot40404/creds/internal/editor"
	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/format"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/gh"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/manifest"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/runner"
	"github.com/elliot40404/creds/internal/session"
	"github.com/elliot40404/creds/internal/vault"
)

var callerOwned = []string{
	"ui/cli.ErrBadChoice",
	"ui/cli.ErrDuplicateField",
	"ui/cli.ErrNeedDash",
	"ui/cli.ErrNoInput",
	"ui/cli.ErrNotTerminal",
	"ui/cli.ErrUnknownType",
	"ui/tui.ErrHomeBroken",
	"ui/tui.ErrNeedPassword",
	"ui/tui.ErrRemoteNoVault",
}

var known = map[string]error{
	"app.ErrAborted":             app.ErrAborted,
	"app.ErrExportFormat":        app.ErrExportFormat,
	"app.ErrFileExists":          app.ErrFileExists,
	"app.ErrImportFile":          app.ErrImportFile,
	"app.ErrJoinUndone":          app.ErrJoinUndone,
	"app.ErrNoEnvPath":           app.ErrNoEnvPath,
	"app.ErrUntrustedProject":    app.ErrUntrustedProject,
	"app.ErrNoField":             app.ErrNoField,
	"app.ErrNoSecret":            app.ErrNoSecret,
	"app.ErrNoVault":             app.ErrNoVault,
	"app.ErrNotDatabase":         app.ErrNotDatabase,
	"app.ErrNotEnv":              app.ErrNotEnv,
	"app.ErrNotRegular":          app.ErrNotRegular,
	"app.ErrPasswordMismatch":    app.ErrPasswordMismatch,
	"app.ErrPhrase":              app.ErrPhrase,
	"app.ErrRecoveryMismatch":    app.ErrRecoveryMismatch,
	"app.ErrRemoteHasVault":      app.ErrRemoteHasVault,
	"app.ErrRemoteNotEmpty":      app.ErrRemoteNotEmpty,
	"app.ErrNoHost":              app.ErrNoHost,
	"app.ErrNoLookup":            app.ErrNoLookup,
	"gh.ErrRepoExists":           gh.ErrRepoExists,
	"gh.ErrBadRepoName":          gh.ErrBadRepoName,
	"gh.ErrNoHost":               gh.ErrNoHost,
	"app.ErrShortPassword":       app.ErrShortPassword,
	"app.ErrStale":               app.ErrStale,
	"app.ErrVaultExists":         app.ErrVaultExists,
	"app.ErrVaultIncomplete":     app.ErrVaultIncomplete,
	"app.ErrWeakPassword":        app.ErrWeakPassword,
	"clipboard.ErrBadHash":       clipboard.ErrBadHash,
	"clipboard.ErrBadMode":       clipboard.ErrBadMode,
	"clipboard.ErrNoTTY":         clipboard.ErrNoTTY,
	"clipboard.ErrNoTool":        clipboard.ErrNoTool,
	"clipboard.ErrTooLarge":      clipboard.ErrTooLarge,
	"app.ErrStrayHome":           app.ErrStrayHome,
	"config.ErrBadDuration":      config.ErrBadDuration,
	"config.ErrNotBool":          config.ErrNotBool,
	"config.ErrNotNumber":        config.ErrNotNumber,
	"config.ErrNoChoices":        config.ErrNoChoices,
	"config.ErrUnknownMode":      config.ErrUnknownMode,
	"config.ErrUnknownSign":      config.ErrUnknownSign,
	"gitsync.ErrSignFailed":      gitsync.ErrSignFailed,
	"config.ErrNotRemovable":     config.ErrNotRemovable,
	"config.ErrUnknownKey":       config.ErrUnknownKey,
	"crypto.ErrWrongKey":         crypto.ErrWrongKey,
	"crypto.ErrWrongSecret":      crypto.ErrWrongSecret,
	"device.ErrCorrupt":          device.ErrCorrupt,
	"device.ErrMismatch":         device.ErrMismatch,
	"device.ErrNoPlugin":         device.ErrNoPlugin,
	"device.ErrNotTrusted":       device.ErrNotTrusted,
	"device.ErrStale":            device.ErrStale,
	"device.ErrUnprotected":      device.ErrUnprotected,
	"device.ErrVaultChanged":     device.ErrVaultChanged,
	"editor.ErrNoEditor":         editor.ErrNoEditor,
	"envfile.ErrBadKey":          envfile.ErrBadKey,
	"envfile.ErrBadRef":          envfile.ErrBadRef,
	"envfile.ErrDuplicateKey":    envfile.ErrDuplicateKey,
	"envfile.ErrEmptyProject":    envfile.ErrEmptyProject,
	"envfile.ErrNUL":             envfile.ErrNUL,
	"envfile.ErrNotDotenv":       envfile.ErrNotDotenv,
	"envfile.ErrNoProject":       envfile.ErrNoProject,
	"envfile.ErrSyntax":          envfile.ErrSyntax,
	"envfile.ErrTooLarge":        envfile.ErrTooLarge,
	"envfile.ErrTrustFile":       envfile.ErrTrustFile,
	"format.ErrOldVersion":       format.ErrOldVersion,
	"format.ErrUnknownVersion":   format.ErrUnknownVersion,
	"fsutil.ErrBusy":             fsutil.ErrBusy,
	"fsutil.ErrTooLarge":         fsutil.ErrTooLarge,
	"anchor.ErrNoAnchor":         anchor.ErrNoAnchor,
	"anchor.ErrRollback":         anchor.ErrRollback,
	"manifest.ErrBadManifest":    manifest.ErrBadManifest,
	"manifest.ErrOverflow":       manifest.ErrOverflow,
	"gitsync.ErrUnverified":      gitsync.ErrUnverified,
	"gitsync.ErrAuthFailed":      gitsync.ErrAuthFailed,
	"gitsync.ErrBadRemote":       gitsync.ErrBadRemote,
	"gitsync.ErrDenied":          gitsync.ErrDenied,
	"gitsync.ErrBadVersion":      gitsync.ErrBadVersion,
	"gitsync.ErrBadMeta":         gitsync.ErrBadMeta,
	"gitsync.ErrBadSide":         gitsync.ErrBadSide,
	"gitsync.ErrConflict":        gitsync.ErrConflict,
	"gitsync.ErrDiverged":        gitsync.ErrDiverged,
	"gitsync.ErrGitStart":        gitsync.ErrGitStart,
	"gitsync.ErrLocked":          gitsync.ErrLocked,
	"gitsync.ErrLockLost":        gitsync.ErrLockLost,
	"gitsync.ErrNoConflict":      gitsync.ErrNoConflict,
	"gitsync.ErrIncomplete":      gitsync.ErrIncomplete,
	"gitsync.ErrNoRemote":        gitsync.ErrNoRemote,
	"gitsync.ErrNotAllowed":      gitsync.ErrNotAllowed,
	"gitsync.ErrPlaintext":       gitsync.ErrPlaintext,
	"gitsync.ErrProtected":       gitsync.ErrProtected,
	"gitsync.ErrPushRejected":    gitsync.ErrPushRejected,
	"gitsync.ErrRemoteExists":    gitsync.ErrRemoteExists,
	"gitsync.ErrRepoNotFound":    gitsync.ErrRepoNotFound,
	"gitsync.ErrUnreachable":     gitsync.ErrUnreachable,
	"render.ErrBadFormatKey":     render.ErrBadFormatKey,
	"render.ErrInvalidConn":      render.ErrInvalidConn,
	"render.ErrUnknownEngine":    render.ErrUnknownEngine,
	"render.ErrUnknownFormat":    render.ErrUnknownFormat,
	"render.ErrUnknownShell":     render.ErrUnknownShell,
	"render.ErrTooBig":           render.ErrTooBig,
	"render.ErrTooLong":          render.ErrTooLong,
	"runner.ErrNoCommand":        runner.ErrNoCommand,
	"session.ErrExpired":         session.ErrExpired,
	"session.ErrNoSession":       session.ErrNoSession,
	"session.ErrUnprotected":     session.ErrUnprotected,
	"vault.ErrBadBucket":         vault.ErrBadBucket,
	"vault.ErrBadPath":           vault.ErrBadPath,
	"vault.ErrDuplicateID":       vault.ErrDuplicateID,
	"vault.ErrDuplicatePath":     vault.ErrDuplicatePath,
	"vault.ErrInvalidEntry":      vault.ErrInvalidEntry,
	"vault.ErrNotFound":          vault.ErrNotFound,
	"vault.ErrRecipientMismatch": vault.ErrRecipientMismatch,
}

func TestEverySentinelClassified(t *testing.T) {
	all := maps.Clone(known)
	maps.Copy(all, osKnown)
	want := slices.Concat(slices.Collect(maps.Keys(all)), callerOwned, osOnly)
	slices.Sort(want)
	got := scanSentinels(t, "..")
	if !slices.Equal(got, want) {
		t.Fatalf("sentinel list drift\ngot  %v\nwant %v", got, want)
	}
	for name, target := range all {
		r, ok := match(target, rules)
		if !ok || !errors.Is(r.Target, target) {
			t.Errorf("%s has no rule", name)
		}
	}
}

func scanSentinels(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && d.Name() == "testutil" {
			return filepath.SkipDir
		}
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}
		for _, name := range fileSentinels(f) {
			out = append(out, filepath.ToSlash(rel)+"."+name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(out)
	return out
}

func fileSentinels(f *ast.File) []string {
	var out []string
	for _, decl := range f.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.VAR {
			continue
		}
		for _, spec := range g.Specs {
			for _, n := range spec.(*ast.ValueSpec).Names {
				if strings.HasPrefix(n.Name, "Err") && n.IsExported() {
					out = append(out, n.Name)
				}
			}
		}
	}
	return out
}
