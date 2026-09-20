package fail

import (
	"github.com/elliot40404/creds/internal/clipboard"
	"github.com/elliot40404/creds/internal/envfile"
	"github.com/elliot40404/creds/internal/gitsync"
	"github.com/elliot40404/creds/internal/render"
	"github.com/elliot40404/creds/internal/runner"
)

const (
	hintStatus  = "run creds status then creds resolve <path> --mine|--theirs"
	hintProject = "run creds env <path>, or add a .creds.toml to the project"
)

var syncRules = []Rule{
	{Target: gitsync.ErrConflict, Code: Conflict, Msg: "merge conflict", Hint: hintStatus},
	{Target: gitsync.ErrDiverged, Code: Conflict, Msg: "local and remote have diverged", Hint: hintStatus},
	{Target: gitsync.ErrNoConflict, Code: NotFound, Msg: "no pending conflict for path", Hint: "run creds status"},
	{Target: gitsync.ErrBadSide, Code: Usage, Msg: "invalid resolution side", Hint: "run creds resolve <path> --mine or --theirs"},
	{Target: gitsync.ErrSignFailed, Code: General, Msg: "could not sign the vault commit", Hint: "set a signing key with git config user.signingkey <path>, or run creds config set sync.sign off"},
	{Target: gitsync.ErrNoRemote, Code: General, Msg: "no remote configured", Hint: "run creds remote add <url>"},
	{Target: gitsync.ErrRemoteExists, Code: Conflict, Msg: "remote already configured", Hint: "run creds remote remove first"},
	{Target: gitsync.ErrBadRemote, Code: Usage, Msg: "invalid remote url", Hint: "run creds remote add <url> with an https, ssh or file url"},
	{Target: gitsync.ErrPushRejected, Code: Conflict, Msg: "push rejected by remote", Hint: "run creds sync"},
	{Target: gitsync.ErrLocked, Code: General, Msg: "sync already running", Hint: "wait for the other sync to finish, then run creds sync"},
	{Target: gitsync.ErrNotAllowed, Code: General},
	{Target: gitsync.ErrPlaintext, Code: General},
	{Target: gitsync.ErrIncomplete, Code: General, Msg: "remote vault is missing files, sync stopped"},
	{Target: gitsync.ErrUnverified, Code: General, Hint: "run creds status, the remote vault is not signed by your identity"},
	{Target: gitsync.ErrProtected, Code: General, Msg: "remote changed the recovery file or vault recipient, sync stopped"},
	{Target: gitsync.ErrBadVersion, Code: General, Msg: "remote vault has a format version this vault cannot take, sync stopped"},
	{Target: gitsync.ErrBadMeta, Code: General, Msg: "remote vault.json is not valid, sync stopped"},
	{Target: gitsync.ErrLockLost, Code: General, Msg: "another creds process took the sync lock", Hint: "run creds sync again"},
	{Target: gitsync.ErrGitStart, Code: General, Msg: "git could not start", Hint: "install git and make sure it is on PATH, then run creds sync"},
	{Target: gitsync.ErrUnreachable, Code: General, Msg: gitsync.ErrUnreachable.Error(), Hint: "check the network and the remote url, then run creds sync"},
	{Target: gitsync.ErrAuthFailed, Code: General, Msg: gitsync.ErrAuthFailed.Error(), Hint: "check the saved login or ssh key for the remote, then run creds sync"},
	{Target: gitsync.ErrRepoNotFound, Code: General, Msg: gitsync.ErrRepoNotFound.Error(), Hint: "check the url in creds status, fix it with creds remote remove and creds remote add <url>"},
	{Target: gitsync.ErrDenied, Code: General, Msg: gitsync.ErrDenied.Error(), Hint: "use an account with write access to the repository, then run creds sync"},
}

var dataRules = []Rule{
	{Target: envfile.ErrTooLarge, Code: Usage},
	{Target: envfile.ErrBadKey, Code: Usage},
	{Target: envfile.ErrSyntax, Code: Usage},
	{Target: envfile.ErrNUL, Code: Usage},
	{Target: envfile.ErrDuplicateKey, Code: Usage},
	{Target: envfile.ErrNotDotenv, Code: Usage, Hint: "pass --format sh, or use creds run to hand the value to the command"},
	{Target: envfile.ErrNoProject, Code: NotFound, Hint: hintProject},
	{Target: envfile.ErrBadRef, Code: Usage},
	{Target: envfile.ErrEmptyProject, Code: Usage},
	{Target: envfile.ErrTrustFile, Code: General, Hint: "delete trust.json in the creds home, then run creds trust"},
	{Target: render.ErrUnknownEngine, Code: Usage},
	{Target: render.ErrInvalidConn, Code: Usage},
	{Target: render.ErrUnknownFormat, Code: Usage},
	{Target: render.ErrBadFormatKey, Code: Usage},
	{Target: render.ErrUnknownShell, Code: Usage},
	{Target: render.ErrTooBig, Code: General, Msg: "the format produced more than 64 KiB of output", Hint: "check the template, a format should print one command, run creds config edit"},
	{Target: render.ErrTooLong, Code: Usage, Msg: "the format template is longer than 8 KiB", Hint: "shorten the template, run creds config edit"},
	{Target: clipboard.ErrBadHash, Code: Usage},
	{Target: clipboard.ErrBadMode, Code: Usage},
	{Target: clipboard.ErrTooLarge, Code: General, Hint: "run creds get <path> --field <name> --show"},
	{Target: clipboard.ErrNoTTY, Code: General, Hint: "run creds copy --native, or run it from a terminal"},
	{Target: clipboard.ErrNoTool, Code: General, Hint: "install a clipboard tool, or use creds copy --osc52"},
	{Target: runner.ErrNoCommand, Code: Usage, Hint: "run creds run [path] -- <command>"},
}
