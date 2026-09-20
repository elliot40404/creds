# creds

Fast terminal password and secrets tool.

- Local first: one encrypted vault on your disk, works offline
- Encrypted with [age](https://age-encryption.org) (post quantum keys)
- Sync through any git remote, even one you do not trust
- Easy recovery: stock `age` can open your vault without creds
- Windows, macOS, Linux. Works over SSH

## Install

### Prebuilt binary

Grab the archive for your system from the
[releases page](https://github.com/elliot40404/creds/releases), unpack it and put
`creds` somewhere on your PATH.

Check that the archive was built by this repo's release workflow. Every release archive and
`checksums.txt` carries a GitHub build provenance attestation:

```sh
gh attestation verify creds_1.0.0_linux_amd64.tar.gz --repo elliot40404/creds
```

It must print `Verification succeeded`. If it does not, do not run the binary.

Then check the archive against `checksums.txt` from the same release. This only catches a broken
download, the attestation above is what proves where the file came from:

```sh
# linux
sha256sum --ignore-missing --check checksums.txt
# macOS
shasum -a 256 --ignore-missing --check checksums.txt
```

```powershell
# Windows
$f = "creds_1.0.0_windows_amd64.zip"
(Get-FileHash -Algorithm SHA256 $f).Hash.ToLower()
Select-String $f checksums.txt
```

The two hashes must match.

### Go

Needs Go 1.27.

```sh
go install github.com/elliot40404/creds/cmd/creds@latest
```

That puts `creds` in your go bin dir (`go env GOBIN`, else `$(go env GOPATH)/bin`).
Add that dir to your PATH if it is not there yet.

### From source

```sh
git clone https://github.com/elliot40404/creds.git
cd creds
go build -trimpath -o bin/creds ./cmd/creds   # bin/creds.exe on Windows
```

With [just](https://github.com/casey/just): `just build`, `just install`, `just uninstall`.

### Check it worked

```sh
creds --version
creds setup        # guided first run, see below
```

### Uninstall

Delete the binary: the one you unpacked, or `$(go env GOPATH)/bin/creds`
(also `just uninstall`).

Your vault is not touched. `creds paths` shows where it lives, delete that dir to
remove it too.

### Extra bits

Needs `git` on your PATH for sync. Linux clipboard needs one of `wl-copy`
(wl-clipboard), `xclip` or `xsel`. Over SSH, or inside a herdr pane on macOS and Linux,
creds copies through the terminal (OSC52), no tool needed. herdr then picks the right
clipboard. A local Windows herdr pane uses the Windows clipboard directly. Under tmux, OSC52
needs `set -g allow-passthrough on`.

Everything else is built in: the vault is one directory of files, there is no
service to run and no account to make.

## Contributing

```sh
just verify       # format, lint, vuln check, tests
just test         # tests only
```

`just test-linux` runs the tests on linux inside a throwaway `golang:1.27`
container. The repo is mounted read only, copied into the container, and the
copy is what gets tested, so nothing writes back to your checkout. Needs
docker; without it the recipe prints a message and does nothing. It is not
part of `just verify`.

Dependencies are vendored, so no download is needed to build.

## First run

Run `creds` with no vault, or `creds setup`, for guided setup. It checks git,
the clipboard tool on linux and that the home dir is writable, then either
creates a new vault or joins one that already exists.

New vault: master password, recovery code shown once and retyped, then sync:

- create a private GitHub repo with `gh` (asks for the name, confirms before creating, never touches repos that exist)
- paste a git url of an empty repository
- skip sync for now

The url is checked first. Unreachable stops with a fix, a repo that already
holds a creds vault points you to join, any other content is refused.

Join: paste the git url of a vault, type the master password, and creds clones
and verifies it. On failure the local copy is removed.

Without a terminal `creds` prints the steps instead of asking.

## Quick start

```sh
creds setup                         # guided first run, or use the steps below
creds init                          # set master password, write down the recovery code
creds add work/github --type login  # asks username, password, url, extra fields
creds list                          # alias: ls
creds search git                    # fuzzy search
creds get work/github               # secrets masked
creds get work/github --show        # secrets shown
creds get work/github --field password
creds copy work/github              # alias: cp, clears after 30s
creds copy work/github --field username
creds copy work/github --osc52      # force the terminal way, or --native for the system clipboard
creds                               # picker: type to filter, Enter copies (same as creds pick)
creds edit work/github
creds rm work/github
creds lock                          # forget the session now
creds version                       # same as creds --version
```

Unlock lasts 15 min idle, 4 h max. Commands ask for the password when needed,
or run `creds unlock` first.

Types: `login api database ssh note command env generic`.

### Guided mode

Add `-i` to any command and creds asks for what is missing. Typed args and flags are kept.

```sh
creds -i                  # pick an action, then answer its questions
creds add -i              # path, type, fields, extra fields (secret y/N)
creds edit -i             # search, pick an entry, edit it
creds list -i             # filter (text, type:login, tag:work), pick, then get/copy/edit/rm
creds status -i           # offers remote add, sync now, then each conflict
creds add web/x --type login -i   # asks only the rest
```

At the end creds prints the same command with flags, for example
`creds add web/x --secret-field password --type login --username bob`.
Secret values are never printed: pass them on stdin, one per line.
`-i` needs a terminal. In scripts drop `-i` and pass flags.

### Databases

Paste a connection string, creds splits it into parts.

```sh
creds add work/db --type database        # pick postgres, redis or mongo, paste the url
creds get work/db --formats              # list formats
creds get work/db --as psql              # PGPASSWORD='...' psql -h host -p 5432 -U user -d app
creds get work/db --as dotenv            # plain PGHOST=... lines for a .env file
creds copy work/db                       # the connection url
creds copy work/db --field password      # just one field
creds get work/db --as psql --shell bash # one-off override of render.shell
```

Formats: postgres `url dotenv env psql pg_dump pg_restore`, redis `url dotenv env redis-cli`,
mongo `url dotenv env mongosh mongodump mongorestore`. `env` is shell flavoured and quoting
follows `render.shell` (`pwsh` on Windows, `bash` elsewhere); `dotenv` is plain `KEY=value`
and the same on every shell.

Copying a database entry with no `--field` or `--as` gives the url, in the TUI (`y`) and on the
CLI. `creds get work/db` with no flags still prints the default field.

`--shell bash|pwsh` on `get` and `copy` overrides `render.shell` for one command. It only makes
sense next to `--as`, so creds says so if you pass it on its own.

creds works out the engine from the connection string, so you rarely pick one. Paste
`mongodb://...` and the TUI add form flips the engine row to `mongo` with a note saying where it
came from; press left or right once and your choice sticks. `creds add -i` asks for the string
first and only asks for the engine when it cannot tell. On the CLI, `--engine` is optional with
`--conn`, and when nothing can be worked out you get `cannot tell the database engine from the
connection string` with the three names to choose from.

### Env vars

```sh
creds import-env .env work/api-env       # .env file -> one env entry
creds env work/api-env                   # print as .env
creds env work/api-env -o .env           # write file (asks first, -y to skip)
creds env work/api-env --format sh       # shell quoting, for eval or source
creds run work/api-env -- npm start      # run with vars set
```

`--format dotenv` (the default) writes lines that node dotenv, python-dotenv and docker read back
exactly: plain values bare, else single quotes, else double quotes with only `\n` escaped. A value
that no quoting keeps intact in all of them (for example one with both `'` and `"`) is refused, use
`--format sh` or `creds run` for it.

With a `.creds.toml` in the project dir, `creds env` and `creds run -- cmd` need no path:

```toml
env = "work/api-env"

[map]
DATABASE_URL = "work/db|url"
API_TOKEN = "work/api:token"
```

`path:field` takes one field, `path|format` takes a database format.

A `.creds.toml` picks which secrets leave the vault, so a cloned repo could ask for any of them.
creds asks before it uses a new or changed file and shows what the file asks for.
Say yes once and it is remembered (file path + SHA-256 in `trust.json`) until the file changes.
With no terminal (CI, pipes) it fails instead. Review the file, then trust it by hand:

```sh
creds trust            # trust ./.creds.toml, prints what it asks for
creds trust ~/code/app # trust another project dir
creds trust list       # every trusted file
creds trust remove <file>
```

Trust only covers which secrets go to the command. While the session is live, any program you
`run` (and its install scripts) can open the whole vault. Only `run` code you trust.
Use `creds run --lock -- npm start` to end the session before the command starts,
or run `creds lock` first, or set a short `session.idle`.

### Sync

Any git remote works. It only ever sees encrypted files.

```sh
creds remote add git@github.com:me/vault.git
creds sync                               # pull, merge, push
creds status                             # remote, ahead/behind, last sync, conflicts
creds resolve work/db --mine             # or --theirs, then creds sync
creds remote remove
```

Second machine:

```sh
creds join git@github.com:me/vault.git   # clone, then unlock with the same password
```

Each change is committed locally. With a remote set, creds also syncs in the background:

- after a command that changed the vault (add, edit, rm, ...)
- on a read (get, list, copy, ...) when the last sync try is older than `sync.stale` (5 min).
  A failed try counts too, so a dead network is retried at most once per 5 min
- never when another sync holds the lock

That background run is `creds sync --quiet`: it never asks anything and only records the
result. You rarely type it yourself.

Background sync never blocks and never prompts (git and ssh run in batch mode). It merges
only while the vault is unlocked. If it fails, later commands other than `sync` and
`status` print `last sync failed: <error>, run creds sync` until a sync works.

### Passwords and recovery

```sh
creds passwd                             # new master password
creds recover                            # forgot password: use recovery code
```

Lost creds itself? See [docs/RECOVERY.md](docs/RECOVERY.md).

### Export and import

```sh
creds export --plain -o backup.json              # asks you to type: yes, export plaintext
creds export --plain --format csv -o backup.csv
creds export --plain -o backup.json --confirm-plaintext "yes, export plaintext"   # no terminal
creds export --encrypted -o vault.tar            # encrypted files only, no unlock
creds import backup.json                         # skip paths that exist
creds import backup.json --overwrite
```

Add `-y` to overwrite an existing export file.

### Scripts and AI agents

`creds help agents` prints the full guide. Short version:

- A person runs `creds unlock` in a terminal first. Scripts use that session.
- Without a terminal creds never prompts. It fails with a fix line instead.
- `--json` prints data as JSON on stdout. Every stderr line is JSON too: `{"note":"..."}` or `{"error":"...","hint":"...","code":3}`.
- Secret values are left out of `get --json` unless you pass `--show` or `--field`.
- Add and edit by flags. Secret values come from stdin, one per line, never from arguments:

```sh
printf '%s\n' "$PW" | creds add web/mail --type login --username bob --secret-field password
printf '%s\n' "$URL" | creds add db/prod --type database --engine postgres --conn -
creds edit web/mail --tag work --field region=eu
creds rm web/mail --yes
```

`--yes` skips the question on `rm`, `env -o`, `export`, `import --overwrite`, `trust`, `resolve` and `join`.
It never skips the plain export phrase: pass `--confirm-plaintext "yes, export plaintext"`.

JSON output:

| Command | Fields |
| --- | --- |
| `get` | `path type name username host url notes tags fields[name value secret] params created updated` |
| `get --field` | `path field value` |
| `get --as` | `path format value` |
| `get --formats` | `path formats` |
| `list`, `search` | `entries[path type name host username tags]` |
| `env` | one key per variable |
| `status` | `remote ahead behind last_sync last_result last_error conflicts` |
| `sync`, `resolve` | `result conflicts` |
| `trust` | `file refs` |
| `trust list` | `trusted[]` |
| `config` | `[key value default doc]` |
| `config get` | `key value` |
| `paths` | `home_env paths[name path exists note]` |
| `version` | `version commit date go os arch` |

Empty fields are left out, lists are always present.

Exit codes:

| Code | Meaning | Fix |
| --- | --- | --- |
| 0 | ok | |
| 1 | error | read the fix line |
| 2 | usage: bad flags or args, or a question needs `--yes` or flags | `creds <cmd> --help` |
| 3 | locked | `creds unlock` in a terminal |
| 4 | not found | `creds search <text>` |
| 5 | conflict | `creds status`, then `creds resolve <path> --mine\|--theirs` |
| 6 | no vault | `creds init` or `creds join <url>` |

`creds run` exits with the code of the command it ran.

### Shell completion

```sh
creds completion bash|zsh|fish|powershell
```

## Config

Optional `config.toml` in the creds home. Values below are the defaults, `render.formats` is an example.

```toml
[session]
idle = "15m"
hard = "4h"

[clipboard]
clear = "30s"

[sync]
stale = "5m"

[render]
shell = "bash"

[render.formats]
"postgres.short" = "psql {{sh .url}}"
```

`render.shell` picks quoting for command formats: `bash` or `pwsh`. Default is `pwsh` on Windows, `bash` elsewhere. Templates can use `sh`, `envset`, `envunset`, `envline`, `flag`, `pathesc`, `queryesc`. Example with `pwsh`: `psql` renders as `$env:PGPASSWORD='x'; psql -h h; Remove-Item Env:PGPASSWORD`.

Edit it by hand, or from the `creds config` commands. Every change is validated before it is saved, so a bad value never lands in the file.

```sh
creds config                                   # every key with its value, and the default when it differs
creds config --json                            # same as data
creds config get session.idle
creds config set session.idle 30m
creds config set render.formats.postgres.short "psql {{sh .url}}"
creds config unset render.formats.postgres.short
creds config edit                              # opens $VISUAL or $EDITOR, validates before saving
creds config -i                                # pick a key from a list
```

Keys:

All keys except `render.formats.*` and `sync.remote` cycle through a short list of values in the
settings TUI, so you pick them with the arrow keys instead of typing. Any other value still goes in
with `creds config set` or by editing the file.

Keys creds works out for itself show what they resolved to and where it came from, in both
`creds config` and the settings screen, so a blank setting is never a blank row:

```
sync.name      Elliot                     (git global)
sync.email     you@users.noreply.github.com  (git global)
vault.machine  my-laptop                  (hostname)
```

Set the key and the note goes away, because your value is then the answer.

| Key | Value | Default |
| --- | --- | --- |
| `session.idle` | lock after this long without use | `15m` |
| `session.hard` | lock this long after unlocking, whatever you do | `4h` |
| `clipboard.clear` | wipe a copied secret after this long | `30s` |
| `sync.stale` | pull again when the last sync is older than this | `5m` |
| `ui.mode` | `fullscreen` or `inline` for the browse TUI | `fullscreen` |
| `ui.altscreen` | swap to a clean screen while the TUI runs, either mode | `true` |
| `ui.height` | rows inline mode may use, 10 to 60 | `15` |
| `sync.name` | name on vault commits, blank uses your global git `user.name` | blank |
| `sync.email` | email on vault commits, blank uses your global git `user.email` | blank |
| `sync.sign` | sign vault commits: `off`, `ssh` or `inherit` | `off` |
| `sync.signkey` | signing key for `sync.sign = ssh`, blank uses your `user.signingkey` | blank |
| `sync.after` | wait this long after the last change before syncing | `2s` |
| `sync.every` | sync at most this often in the background | `30s` |
| `vault.history` | keep this many previous versions of each entry, 0 turns it off | `3` |
| `vault.machine` | name recorded on each edit, blank uses the hostname | blank |
| `render.shell` | `bash` or `pwsh` | `pwsh` on Windows, else `bash` |
| `render.formats.<engine>.<name>` | template override, for example `render.formats.postgres.psql` | none |

Durations must be at least `1s` and `session.idle` must not exceed `session.hard`. Only `render.formats.*` keys can be removed with `unset`.

`creds` and `creds pick` take `--inline`, `--fullscreen`, `--height <rows>` and `--alt-screen=false`
to override the `ui.*` settings for one run. Inline draws in at most `--height` rows under your
prompt instead of filling the terminal; turning off the alt screen leaves the UI in scrollback when
you quit. The two are independent, so `--inline --alt-screen=false` is a small box that stays put.
Heights outside 10 to 60 are clamped.

In the add form, `tab` on the path row completes the folder you are typing, shown as dim ghost
text ahead of the cursor. `wo` then `tab` gives `work/`, `tab` again goes a level deeper. With no
ghost, `tab` moves to the next field as usual. Typing a path that already exists shows the same
duplicate message you would get on save, and it clears as soon as you change the path. The edit
form has no completion. On the CLI, `creds add <TAB>` offers folders rather than whole paths, and
`creds add -i` lists the folders next to the question.

Any list you pick from (formats, entry type, first-run setup) numbers its first nine rows, so
`1` to `9` picks one without arrow keys.

On the format list (`f`) the numbers also pick a shell. `1` to `9` copies for your `render.shell`,
`shift+1` to `shift+9` copies the same format for the other shell. On Windows that is `1` for
`pwsh` and `shift+1` for `bash`, on Linux and macOS it is the other way round. Arrows and `enter`
ask which shell instead, and skip the question for formats like `url` and `dotenv` that come out
the same either way.

The TUI shows the same list on `,`, and below it the sync remote (`sync.remote`: Enter changes
it, `x` removes it) and one row per trusted project file (`x` forgets it). The file list from
`creds paths` sits on top of that. On a config row with a list of values, the row shows
`‹ value ›` and left and right change it in place, wrapping at both ends; a value typed by hand,
like `session.idle = "7m"`, snaps to the nearest one in the list. Rows that are free text
(`sync.remote`, `render.formats.*`) still open a text box on Enter. `r` puts the default back and
`a` adds a format override, with a preview on sample data.

### Signing the vault's commits

Off by default, because turning it on for someone with no key would fail every commit, and in
creds a failed commit is a failed write.

```
creds config set sync.sign ssh                       # sign with SSH
creds config set sync.signkey ~/.ssh/id_ed25519.pub  # optional, else your user.signingkey
creds config set sync.sign inherit                   # let your own git config decide
```

- `off` forces `commit.gpgsign=false`, so a global `commit.gpgsign = true` never applies to the
  vault. That is the point of the setting.
- `ssh` forces `commit.gpgsign=true` and `gpg.format=ssh`, and `user.signingkey` too when
  `sync.signkey` is set.
- `inherit` passes none of the three and your own git config decides. **Merge commits are not
  signed in this mode**: they are made with `git commit-tree`, which signs only when creds passes
  `-S`, and creds only does that for `ssh`.

Signed is not the same as verified. A git host marks a signature Verified only when the committer
email is a verified address on your account, so creds commits as your global git identity:

1. `sync.name` and `sync.email` when you set them
2. your global git `user.name` and `user.email`
3. `creds <creds@localhost>` when git has no global identity, so a machine without one still
   commits rather than failing the write

On GitHub you must also add the key a second time with **Key type: Signing Key**. An
authentication key does not count, and this is the usual reason a correctly signed commit still
shows Unverified.

Your name and email end up in the vault repo's commit metadata, which the host can read even
though it cannot read the entries. To keep the old anonymous identity:

```
creds config set sync.email creds@localhost
creds config set sync.name creds
```

To check a signature locally you also need an allowed signers file, which is separate from
signing:

```
git config --global gpg.ssh.allowedSignersFile ~/.ssh/allowed_signers
echo "$(git config --global --get user.email) $(cat ~/.ssh/id_ed25519.pub)" >> ~/.ssh/allowed_signers
```

The email there must match the committer. `git -C <vault> cat-file commit HEAD | grep gpgsig`
tells you a commit is signed without any of that setup.

If signing fails, the write fails and says so. creds never quietly writes an unsigned commit when
you asked for a signed one. Normal writes commit in the foreground so you see it straight away; a
merge commit made by the background sync puts the error in `creds status`.

creds keeps `--no-verify` and an empty `core.hooksPath` so your repo hooks never run against the
vault. Neither blocks signing, hooks run before it.

### When it syncs

Adding or changing an entry pushes on its own. The browse TUI syncs in its own process `sync.after`
(2s) after the last change, so adding three entries in a row is one push, not three. The header
shows `◌ syncing` while it runs, then `● synced just now`. `s` still syncs on demand and says
`sync already running` rather than starting a second one.

On the CLI each command is its own process, so it spawns a detached `creds sync --quiet`. That
child waits `sync.after` before taking the lock, so a burst coalesces there too, and it waits a few
seconds for a lock another creds is holding instead of giving up straight away.

`sync.every` (30s) caps how often either path starts a background sync. After a failure creds backs
off to five times that, capped at `sync.stale`, so a broken remote does not spawn a process per
keystroke. The failure line stays on screen until a sync succeeds.

### Filtering and sorting

`creds search` and the `/` box in the TUI take `type:`, `engine:` and `tag:` tokens anywhere in
the query. Anything else is fuzzy matched as before.

```
creds search type:database engine:postgres      # both must hold
creds search tag:work tag:personal              # either tag
creds search type:login prod                    # a filter plus fuzzy text
creds list --sort updated                       # newest edit first
creds search db --sort created
```

Different keys are ANDed, repeats of one key are ORed. Values must match exactly, ignoring case,
so `type:DB` finds nothing. A key creds does not know, like `foo:bar`, stays part of the fuzzy
text, so urls keep working.

In the TUI, `t` cycles the `type:` token inside the `/` box, so it composes with whatever else is
typed and you can see what it did. `o` cycles the order and the count at the top right shows it
(`12/340  sort updated`). Type `engine:` yourself, there is no key for it.

### Previous versions

creds keeps the last `vault.history` versions of each entry's values, 3 by default, with the name
of the machine that made the edit. The detail screen shows `created`, `updated` and the machine,
and `h` opens the list of previous versions; pick one to see its values, `r` reveals a secret.
`creds get` prints the same rows and how many versions are kept.

`vault.machine` overrides the recorded name, which is the hostname otherwise.

**A rotated secret stays readable in the vault for `vault.history` more edits of that entry.**
That is the point of the feature and the cost of it. Set `vault.history` to `0` to turn it off,
which also clears what is already stored the next time each entry is written.

This is vault format 2. creds reads a format 1 vault and writes it back as format 2. An older
creds refuses a format 2 vault and tells you to install the latest release.

### Where the files are

```sh
creds paths          # every creds file, its full path and whether it exists
creds paths --json   # same as data
```

It lists the creds home, `config.toml`, the vault dir with its remote, the session with the time left, `state.json`, `trust.json` and `sync.lock`. When `CREDS_HOME` is set, it says so. The TUI settings screen shows the same list.

## Files

Home is `~/.config/creds` on all systems. Set `CREDS_HOME` to use another dir.

```
~/.config/creds/
  config.toml  session  state.json  sync.lock  trust.json
  vault/       git repo: vault.json, identity.pw.age, identity.recovery.age, entries/00.enc .. 0f.enc
```

## If your vault may have leaked

Rotate every credential stored inside the vault. Then delete the remote repository and run
`creds init` against a fresh one. There is no `rekey` command: re-encrypting in place cannot reach
clones, forks, provider backups or another device's reflog, and once the credentials inside are
rotated the stolen vault is worthless. See [docs/SECURITY.md](docs/SECURITY.md).

## Docs

- [docs/FORMAT.md](docs/FORMAT.md): exact file formats
- [docs/RECOVERY.md](docs/RECOVERY.md): get secrets back with stock age
- [docs/SECURITY.md](docs/SECURITY.md): threat model and known leaks

## License

MIT
