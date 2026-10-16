package fail

import (
	"slices"

	"github.com/elliot40404/creds/internal/anchor"
	"github.com/elliot40404/creds/internal/app"
	"github.com/elliot40404/creds/internal/config"
	"github.com/elliot40404/creds/internal/crypto"
	"github.com/elliot40404/creds/internal/device"
	"github.com/elliot40404/creds/internal/editor"
	"github.com/elliot40404/creds/internal/format"
	"github.com/elliot40404/creds/internal/fsutil"
	"github.com/elliot40404/creds/internal/gh"
	"github.com/elliot40404/creds/internal/manifest"
	"github.com/elliot40404/creds/internal/session"
	"github.com/elliot40404/creds/internal/sshdir"
	"github.com/elliot40404/creds/internal/update"
	"github.com/elliot40404/creds/internal/vault"
)

const (
	hintUnlock  = "run creds unlock"
	hintSearch  = "run creds search <text>"
	hintInit    = "run creds init or creds join <url>"
	hintRestore = "run creds sync or restore the vault files with git, do not run creds init"
	ThisCommand = "the command"
	hintRerun   = "run " + ThisCommand + " again"
)

var rules = slices.Concat(vaultRules, appRules, syncRules, dataRules, osRules)

var vaultRules = []Rule{
	{Target: app.ErrNoVault, Code: NoVault, Hint: hintInit},
	{Target: app.ErrVaultExists, Code: Conflict, Hint: "run creds unlock to use the existing vault"},
	{Target: app.ErrVaultIncomplete, Code: General, Hint: hintRestore},
	{Target: session.ErrNoSession, Code: Locked, Msg: "vault is locked", Hint: hintUnlock},
	{Target: session.ErrExpired, Code: Locked, Msg: "session expired", Hint: hintUnlock},
	{Target: session.ErrUnprotected, Code: Locked, Msg: "session file was readable by others and was dropped", Hint: hintUnlock},
	{Target: device.ErrNotTrusted, Code: General, Hint: "run creds device trust --touchid or --identity <file>"},
	{Target: device.ErrStale, Code: General, Hint: hintUnlock + " with the master password"},
	{Target: device.ErrVaultChanged, Code: General, Hint: "run creds device trust again"},
	{Target: device.ErrUnprotected, Code: General, Hint: "run creds device trust again"},
	{Target: device.ErrCorrupt, Code: General, Hint: "run creds device trust again"},
	{Target: device.ErrMismatch, Code: Usage, Hint: "run creds device trust --identity <file> with the file the plugin keygen made"},
	{Target: device.ErrBadKey, Code: Usage, Hint: "run creds device trust --identity <file> with the file the plugin keygen made"},
	{Target: device.ErrNoTouchID, Code: Usage, Hint: "run creds device trust --identity <file> with another age plugin, like age-plugin-yubikey or age-plugin-tpm"},
	{Target: device.ErrNoPlugin, Code: General, Hint: "install the age plugin on PATH, or run creds device untrust"},
	{Target: app.ErrJoinUndone, Code: General, Hint: "run creds join <url> again"},
	{Target: app.ErrRemoteHasVault, Code: Conflict, Hint: "run creds join <url> to use that vault"},
	{Target: app.ErrRemoteNotEmpty, Code: Conflict, Hint: "pick an empty repository, creds never writes over other content"},
	{Target: app.ErrNoHost, Code: General, Hint: "paste a git url in creds setup instead"},
	{Target: app.ErrNoLookup, Code: General, Hint: "report this bug, then run creds init and creds remote add <url>"},
	{Target: gh.ErrNoHost, Code: General, Hint: "install gh and run gh auth login, then run creds setup again, or paste a git url instead"},
	{Target: gh.ErrRepoExists, Code: Conflict, Hint: "pick another name, creds never uses an existing repository"},
	{Target: gh.ErrBadRepoName, Code: Usage, Hint: hintRerun + " with a name of letters, digits, dot, dash or underscore"},
	{Target: update.ErrUnreachable, Code: General, Hint: "check the network, then run creds update again"},
	{Target: update.ErrRateLimited, Code: General, Hint: "GitHub limits release checks per hour, wait an hour then run creds update again"},
	{Target: update.ErrBadRelease, Code: General, Hint: "GitHub sent release data creds could not use, try creds update again later"},
	{Target: update.ErrChecksum, Code: General, Hint: "the download did not match the release checksums, nothing changed, try creds update again later"},
	{Target: update.ErrNoAsset, Code: General, Hint: "no release file for this system, build creds with go install github.com/elliot40404/creds/cmd/creds@latest"},
	{Target: update.ErrDevBuild, Code: Usage, Hint: "update a source build of creds with go install github.com/elliot40404/creds/cmd/creds@latest or just install"},
	{Target: crypto.ErrWrongSecret, Code: General, Hint: "check the password and try again, or run creds recover"},
	{Target: crypto.ErrWrongKey, Code: General, Hint: "a vault file does not belong to this vault, run creds status and check the remote"},
	{Target: vault.ErrRecipientMismatch, Code: General, Hint: "the vault files do not match this identity, restore them with git or run creds join <url> in a new CREDS_HOME"},
	{Target: vault.ErrNotFound, Code: NotFound, Hint: hintSearch},
	{Target: vault.ErrDuplicatePath, Code: Conflict, Hint: "pick another path or run creds edit <path>"},
	{Target: app.ErrStale, Code: Conflict, Hint: "run creds get <path> to see the current entry, then edit it again"},
	{Target: vault.ErrDuplicateID, Code: Conflict},
	{Target: vault.ErrBadPath, Code: Usage, Hint: hintRerun + " with a path like web/github: no leading / or -, no . or .. parts, no empty parts"},
	{Target: vault.ErrInvalidEntry, Code: General},
	{Target: vault.ErrBadBucket, Code: General},
	{Target: format.ErrUnknownVersion, Code: General, Hint: "install the latest creds release"},
	{Target: anchor.ErrRollback, Code: Conflict, Hint: "the remote is serving an older vault than this machine already trusted, run creds status and check the remote before syncing again"},
	{Target: anchor.ErrNoAnchor, Code: General, Hint: "run creds sync to record the vault state this machine trusts"},
	{Target: manifest.ErrBadManifest, Code: General, Hint: "the vault files do not match their signed manifest, run creds status and check the remote"},
	{Target: manifest.ErrOverflow, Code: General, Hint: "run creds init against a fresh remote, this vault has no generations left"},
	{Target: sshdir.ErrStateCorrupt, Code: General, Hint: "remove the ssh.json shown by creds paths, creds then asks again about keys and changed files"},
	{Target: fsutil.ErrTooLarge, Code: General, Hint: "the file or the remote data is far bigger than a vault should ever be, run creds status and check the remote"},
	{Target: format.ErrOldVersion, Code: General},
	{Target: fsutil.ErrBusy, Code: General, Hint: "another creds command is busy with the same file, try again"},
	{Target: sshdir.ErrSymlink, Code: General, Hint: "creds does not follow symlinks in the ssh dir, put the real file there instead"},
	{Target: sshdir.ErrSyntax, Code: General, Hint: "fix the line in your ssh config so creds can read it"},
	{Target: sshdir.ErrToken, Code: General, Hint: "write the full key path without % or ${} tokens so creds can find the key"},
	{Target: sshdir.ErrOutside, Code: General, Hint: "creds only syncs files inside the ssh dir, move the file there to sync it"},
	{Target: sshdir.ErrIncludeLoop, Code: General, Hint: "remove the Include that points back to an earlier ssh config file so creds reads each file once"},
}

var appRules = []Rule{
	{Target: app.ErrNoField, Code: NotFound, Hint: "run creds get <path> to list fields"},
	{Target: app.ErrNoSecret, Code: Usage, Hint: "run creds get <path> to list fields, then pass --field <name>"},
	{Target: app.ErrNotEnv, Code: Usage, Hint: "run creds list to find an env entry"},
	{Target: app.ErrNotDatabase, Code: Usage, Hint: "run creds list to find a database entry"},
	{Target: app.ErrNoEnvPath, Code: Usage, Hint: hintProject},
	{Target: app.ErrUntrustedProject, Code: Usage, Hint: "review .creds.toml, then run creds trust, or run creds trust list to see trusted project files"},
	{Target: app.ErrExportFormat, Code: Usage, Hint: "run creds export --plain|--encrypted -o <file>"},
	{Target: app.ErrFileExists, Code: Conflict, Hint: "rerun the creds command with another output file, or with --yes to overwrite"},
	{Target: app.ErrPhrase, Code: General, Hint: hintRerun + " and type the phrase exactly as shown"},
	{Target: app.ErrNotRegular, Code: General},
	{Target: app.ErrImportFile, Code: Usage, Hint: "pass a file made by creds export --plain --format json"},
	{Target: app.ErrShortPassword, Code: Usage, Hint: hintRerun + " with a longer password, like four random words"},
	{Target: app.ErrWeakPassword, Code: Usage, Hint: hintRerun + " with a stronger password, like four random words"},
	{Target: app.ErrPasswordMismatch, Code: Usage, Hint: hintRerun + " and type the same password twice"},
	{Target: app.ErrRecoveryMismatch, Code: Usage, Hint: hintRerun + " and type the recovery code exactly as shown"},
	{Target: app.ErrAborted, Code: General},
	{Target: app.ErrStrayHome, Code: Conflict, Hint: "move or delete the listed files, or run creds setup in an empty CREDS_HOME"},
	{Target: app.ErrSSHPath, Code: Usage, Hint: "creds syncs only plain paths inside the ssh dir: no .., /, \\, : or drive letters, rename the file or run creds rm <path>"},
	{Target: app.ErrSSHFile, Code: General, Hint: "creds syncs only ssh config, private keys and .pub files as UTF-8 text up to 64 KiB, run creds get <path> to check the entry"},
	{Target: app.ErrSSHHost, Code: Usage, Hint: "run creds ssh add-host group/name --host 10.0.0.5, names use letters, digits, . _ -, --key names a private key file in ~/.ssh"},
	{Target: app.ErrSSHKey, Code: Usage, Hint: "creds ssh add-key takes a private key file up to 64 KiB, not a .pub, config or link, like creds ssh add-key ~/Downloads/id_ed25519 --name work_key"},
	{Target: app.ErrSSHKeygen, Code: Usage, Hint: "run creds ssh keygen id_work, names use letters, digits, . _ - and no .pub, --comment is one line of text"},
	{Target: app.ErrSSHPassphrase, Code: Usage, Hint: "type a passphrase twice, or run creds ssh keygen <name> without --passphrase"},
	{Target: app.ErrSSHExists, Code: Conflict, Hint: "pick another name, or edit the file in ~/.ssh and run creds ssh push"},
	{Target: config.ErrUnknownKey, Code: Usage, Hint: "run creds config to list every key"},
	{Target: config.ErrNotRemovable, Code: Usage, Hint: "run creds config set <key> <value> instead"},
	{Target: config.ErrBadDuration, Code: Usage, Hint: "run creds config set <key> 15m, any number and unit works, like 90s or 4h"},
	{Target: config.ErrNotBool, Code: Usage, Hint: "run creds config set <key> true, or false"},
	{Target: config.ErrNotNumber, Code: Usage, Hint: "run creds config set <key> 15, a whole number"},
	{Target: config.ErrNoChoices, Code: Usage, Hint: "type a value for that key, run creds config set <key> <value>"},
	{Target: config.ErrUnknownMode, Code: Usage, Hint: "run creds config set ui.mode fullscreen, or inline"},
	{Target: config.ErrUnknownSign, Code: Usage, Hint: "run creds config set sync.sign off, ssh, or inherit"},
	{Target: config.ErrUnknownUpdate, Code: Usage, Hint: "run creds config set update.mode hint, install, or off"},
	{Target: editor.ErrNoEditor, Code: General, Hint: "set EDITOR to your editor, then run creds config edit"},
}
