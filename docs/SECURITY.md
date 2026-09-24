# creds security

What creds protects, from whom, and where it leaks. Plain words, no promises it cannot keep.
File details are in [FORMAT.md](FORMAT.md).

## Keys in one picture

```
master password --scrypt logN 18--> identity.pw.age       \
recovery code   --scrypt logN 18--> identity.recovery.age  > both hold the same vault identity
                                                           /
vault identity (age ML-KEM-768 + X25519) --> opens entries/00.enc .. 0f.enc
vault identity --HKDF--> MAC key --> stamps each bucket (MAC inside the envelope)
```

- Crypto is age (filippo.io/age), not hand made
- Password: at least 12 chars plus a built in weak password check, weak passwords are always refused on set
- Vault file set signed by `manifest.age`, and the accepted generation recorded in `anchor.json`,
  so an untrusted remote cannot replay an older vault state past a machine that has seen a newer one
- Recovery code: 150 random bits, shown once, must be typed back at init

## Who we defend against

### 1. Untrusted git remote

Example: GitHub, a friend's server, a stolen backup.

Protected:
- Entry content, paths, names, usernames, count of entries per file
  (all inside age, padded)
- Commit message is always `update vault` or `Merge remote changes`
- Commit author is **not** anonymous by default. creds uses your global git `user.name` and
  `user.email`, falling back to `creds <creds@localhost>` only when git has no global identity.
  So anyone who can read the repo learns who edits the vault and when. This is a deliberate
  change: a git host only marks a signature verified when the committer email is one of your
  verified addresses, so a fixed fake identity made `sync.sign` useless for that purpose.
  Set `sync.name` and `sync.email` to pin any identity you like, including the old fixed one:
  `creds config set sync.email creds@localhost`
- Fake buckets: anyone can age encrypt to the public recipient, but they cannot make the MAC
- Swapped buckets: slot inside must match the file name, each entry must hash to that slot
- Swapped identity: recipient must match `vault.json`, and a new `identity.pw.age` only
  helps an attacker who knows your password
- Plain files: `.gitignore` allowlist plus a commit guard that refuses any `.age`/`.enc`
  file without the age header
- Bad trees: clone, fast-forward and merge all check the remote tree first. Only vault
  files, only plain files (no symlinks, no submodules, no `.gitattributes`), max 64 MiB each.
  Anything else stops the sync and nothing is checked out
- Fast-forward opens all 16 incoming buckets with the MAC check once any bucket changed,
  and refuses two entries with the same path, before it moves the tree. That needs a live session: a quiet background sync with no session records
  `needs unlock` and leaves the tree as is until you unlock and sync again
- Incoming `vault.json` must pass the same checks as the local loader (1 MiB cap, no unknown
  members, valid `created`, same `format_version` as the local one), else the sync stops
- Vault files are read with a size cap and must be regular files
- Migration backup (`.migrate-backup/`) is only restored as known vault files, and only
  inside the vault dir. Anything else stops with an error

Leaks and limits:
- Offline guessing: the remote has `identity.pw.age`. A weak password can be brute forced
  (scrypt makes each guess slow, not impossible). Use a long password
- Timing: commit times show when you change things. Which bucket file changed shows that
  one entry in that 1 of 16 group changed
- Rough size: file size shows the padded size (4K, 8K, 16K ...). Many entries in one bucket
  push it to a bigger size, so size hints at how many entries or how big they are
- Replay: the MAC signs the content, not a version. Someone with write access can put back
  an OLD bucket that you once wrote (from git history). creds accepts it. Effect: entries in
  that bucket go back in time, new ones vanish, deleted ones come back. The 3 way merge
  usually shows this as a change from "theirs", but nothing blocks it.
  Same for an old `identity.pw.age`: the new password stops working and the old one works.
  When a sync changes `identity.pw.age`, creds warns on later commands until you unlock with
  the password or run `passwd`. Example: you ran `passwd` on laptop, desktop warns once it syncs
- Delete / hold back: the remote can refuse pushes or hide new commits. creds cannot tell
- Size: git downloads the whole clone or fetch before creds checks it. A huge remote can fill
  your disk or take long (up to the 2 min git timeout). A failed clone is removed. Accepted
- Stock age (manual recovery) does not check the MAC

### 2. Git history is forever

- A deleted entry stays in old commits. Anyone with the identity can read it
- `passwd` only rewrites `identity.pw.age`. Old commits keep the old wrapped file, so the
  OLD password still unlocks the SAME identity, which opens ALL current data too.
  Changing the password does not lock out someone who knew the old one
- Same for the recovery code: there is no rotation yet
- If the password, code, or identity leaked: make a new vault (`init` in a new home,
  `export --plain` + `import`), push to a NEW remote, delete the old one
- No history purge command yet

### 3. Local attacker

Other user on the same machine, or someone who gets your disk.

Protected:
- Vault files are encrypted at rest. Dirs `0700`, files `0600`
- Windows: creds dirs (home, vault) and every file creds writes get a protected access list
  (you and SYSTEM only, nothing inherited). Files get it when they are created, so no other
  account can open them in between. Existing creds dirs with a wider list are fixed on the next
  write. Covers the home dir, vault, session, state, export and `env -o` files
- Unix: existing creds dirs with group or other bits are set back to `0700` on the next write
- creds warns if the home dir, vault or session is open to others: on unix group or other bits,
  on Windows read, write, delete or access list rights for any account other than you, SYSTEM or
  Administrators (with an `icacls` fix), or an owner that is another account (with a `takeown`
  fix). If the check itself fails, creds warns too
- Atomic writes: a crash leaves the old file or the new file, never half of one

Leaks and limits:
- `session` file holds the identity in plain text while unlocked (default 15m idle,
  4h max). Any program running as you can read it and open the whole vault.
  An expired session file stays on disk until the next creds command or `creds lock`.
  Run `creds lock` when you step away
- The TUI loads the entry again on reveal and edit, and checks the session before the edit
  form shows a masked value (`ctrl+r`, or `ctrl+t` on a secret custom field), so after the
  session ends it asks for the password first. A field already revealed stays on screen until you move or leave
- Malware running as you can also read your keystrokes, clipboard and memory. creds
  cannot defend against that
- Windows: dirs above the creds home (for example the parent of `CREDS_HOME`) keep their
  access list. If another account owns the home dir it can change the list back at any time,
  creds only warns
- Go memory is mostly not wiped. The bucket MAC key byte slices are zeroed once a vault or merge
  is done with them, but the identity and entry values live in Go strings, which cannot be wiped,
  and the garbage collector may have copied any of it. Secrets and keys may stay in RAM, swap, or
  crash dumps until the OS reuses the memory
- `export --plain` and `env -o file` write secrets in plain text (`0600`). A typed phrase
  guards plain export. Delete those files when done
- `export --plain --format csv` is a partial view: it drops notes, tags and database params
  and `import` does not read it. creds says so on stderr. Use `--format json` for a real backup
- `run` puts secrets into the child's environment. Other programs running as you may see
  them (for example `/proc/<pid>/environ` on Linux)
- `state.json` and `sync.lock` are plain but hold no secrets. `state.json` can hold entry
  paths of sync conflicts

### 4. Shoulder surfing and screen capture

- `get` masks secret fields. Only `--show`, `--field`, `--as` print them
- Password and recovery code prompts do not echo
- Secrets are never taken from command line args, so they do not land in shell history
- Printed secrets stay in terminal scrollback and tmux history
- The recovery code is shown on screen once at `init`

### 5. Clipboard

- `copy` and `pick` put a secret on the clipboard, then a detached helper clears it after
  30s (`clipboard.clear`). With the system clipboard it only clears if the clipboard still holds
  the same value. The helper gets only a hash of the value, not the value
- Windows: the copy is marked to skip clipboard history (Win+V) and cloud sync. The helper breaks
  away from the parent job, so closing an ssh session or terminal does not kill it (it falls back
  to staying in the job when the job refuses that)
- Clipboard managers on macOS and Linux may still keep a copy. macOS Universal Clipboard
  (Handoff) syncs the copy to your other Apple devices, since `pbcopy` cannot mark it as concealed
- Over SSH creds uses OSC52 (text goes through the terminal). Inside a herdr pane it does the same
  on macOS and Linux, and on Windows only when an ssh variable is also set. A local Windows herdr
  pane uses the system clipboard, so history skip and clearing work there
- OSC52 clearing is blind: the terminal cannot report what the clipboard holds, so the clear sends
  an empty copy after 30s even if you copied something else since. Some terminals ignore it, and
  Windows has no OSC52 clear
- tmux drops the OSC52 copy unless `set -g allow-passthrough on` (off by default since tmux 3.3),
  and creds still says `Copied`. tmux may also keep its own paste buffer
- OSC52 is written to the controlling terminal, or to stderr only when stderr is a terminal.
  With no terminal (cron, CI, `2>file`) copy fails, so the secret never lands in a log
- On Linux creds picks `wl-copy` when `WAYLAND_DISPLAY` is set, else `xclip`/`xsel` when `DISPLAY`
  is set, and tries the next installed tool when one fails
- If the helper cannot start, creds warns and the secret stays until you copy something else. If
  the helper runs but the clear fails, the error is kept in `state.json` and the next command
  prints `warning: clipboard was not cleared after the last copy`
- Any program running as you can read the clipboard while the secret is on it

## Guided setup and gh

- `creds setup` runs before any vault exists. It checks the home dir, `git`, and on Linux a
  clipboard tool, then creates or joins a vault
- A url you type is probed first: `git ls-remote` and a shallow `--filter=blob:limit=1m` fetch
  in a private temp dir, with a 30s timeout. Only the file names are read, nothing is kept.
  A remote that already holds a vault says "join instead", a remote with other content is refused
- The `gh` path only runs when `gh` is on PATH and `gh auth login` was done. creds asks before
  `gh repo create <name> --private`, refuses names that are not letters, digits, dot, dash or
  underscore, and never touches a repository that already exists. A gh failure that is not
  "not found" stops setup, it is not read as a free name
- creds never reads or stores your GitHub token; `gh` holds it
- Settings written at the end (`config.toml`) hold no secrets

## Settings surface

- `config.toml` holds no secrets, only durations, a shell name and output templates. A template
  can name `.password`, but the value is filled in at print time, never stored in the file
- `creds config set` and the TUI settings screen validate the whole config before writing.
  A bad value is refused and the old file stays. Writes are atomic, mode `0600`
- `creds config edit` copies the settings into a fresh temp dir (`0600` file), runs your
  `$VISUAL`/`$EDITOR` (or `notepad`/`vi`/`nano`), then validates what came back and removes the
  temp dir. Your editor is a program you trust already: it runs as you and can read the session
  file. An editor that writes swap or backup files leaves those in that temp dir, which creds
  deletes when it exits, not on a kill
- `creds paths` and the TUI file list print paths, whether each file exists, and the session
  time left. No secrets. The remote URL goes through the same `user:password@` hiding as `status`
- `creds trust list` prints the project file paths you approved, `creds trust remove <file>`
  forgets one. Both only touch `trust.json`, which holds paths and hashes, no secrets.
  Removing a trusted file means the next `env`/`run` asks again

## Programs creds trusts

creds runs `git`, `gh`, the clipboard tools (`wl-copy`, `xclip`, `xsel`, `pbcopy`) and age
plugins (`age-plugin-<name>`, only after `creds device trust`) as
resolved by your PATH at the time of the call, with the value on stdin, never through a shell.
A writable directory earlier in your PATH can therefore see secrets you copy. Keep PATH clean.

## Device unlock (Touch ID, YubiKey, TPM)

`creds device trust` seals a copy of the vault identity to an age plugin key and keeps it in
`device.json` in the creds home. After that a touch unlocks the vault instead of the password.

- The copy is local only. It is never in `vault/`, never committed, never pushed. Another machine
  with the same vault still needs the password
- Trust always asks the master password, even with a live session, and opens the new copy once
  before saving it
- `device.max_age` (default `72h`) makes the password needed again now and then, so you keep
  remembering it. `0` turns that off and creds warns
- If the vault key changes (a new vault, or a remote with another vault), the copy no longer
  matches and is deleted with a warning
- `creds passwd` does not revoke the device. It changes only the password file, the vault identity
  is the same, so the device copy still opens the vault. passwd and recover ask whether to untrust
  the device too. Say yes if you think the machine or the plugin key is not safe
- Touch ID (`--touchid`) makes the key with `age-plugin-se keygen --access-control
  current-biometry`. Adding or removing a fingerprint breaks the key: unlock with the password and
  run `creds device trust --touchid` again. The key lives in the Secure Enclave and is never written
  to a file by creds
- The Secure Enclave key is P-256, not post-quantum. The vault itself stays post-quantum (ML-KEM),
  only the local device copy is not. A future quantum attacker also needs `device.json` from your disk
- The plugin is found on PATH as `age-plugin-<name>`, the age plugin client cannot pin a path.
  Anything that can put a program early on your PATH can already replace `creds` itself
- Malware running as you can ask the plugin to open the copy. With Touch ID, or a YubiKey with a
  touch policy, the plugin still wants a touch each time, so it gets the identity only if you touch.
  A plugin key with no touch or PIN policy gives no such guard

## Command entries are never run

`command` type entries are text. creds shows or copies them, never runs them.
Only `creds run -- <cmd>` runs a program, and the program comes from you on the command
line, not from the vault. A test makes sure only that one command can start programs.

## Sync safety

- System `git` runs with hooks off, no signing, `core.symlinks=false`,
  `transfer.fsckObjects=true`, and never asks for input (`GIT_TERMINAL_PROMPT=0`)
- Only `https`, `ssh` and `file` remotes work (`protocol.allow=never` plus those three).
  Example: `ext::...`, `fd::3`, `git://` and `http://` are refused
- Git env vars that could redirect the repo (`GIT_DIR`, `GIT_WORK_TREE`, ...) are dropped
- Merge works per entry. Real conflicts are saved and shown, never forced.
  You pick with `creds resolve <path> --mine|--theirs`
- Every remote state creds is about to adopt is checked against `manifest.age` first: the file set
  must be exactly the one the manifest signs, byte for byte. A remote that mixes an old bucket with
  today's other buckets is refused, even though each file on its own is still valid
- The accepted generation is kept in `anchor.json`. A fast forward or fresh clone to a state with a
  lower generation is a rollback and is refused
- A diverged remote is checked against the merge base instead, since the other machine may simply
  have made fewer commits than this one. Its generation must be higher than the base's. Example: an
  attacker builds a branch on top of commit C that holds an older genuine vault. Its generation is
  at or below C's, so the merge is refused instead of silently undoing newer changes. A remote with
  no common history at all must pass the anchor check. A good merge writes a manifest with both
  sides as parents
- Without a live session (background `sync --quiet`) creds cannot check the manifest, so it takes
  nothing new: any change to any vault file or to `manifest.age` means `needs unlock`, and the next
  unlocked sync checks it first. Example: a remote that swaps only `identity.pw.age` is not adopted
- Before a local write signs a new manifest, the last committed state is checked against its own
  manifest. A tampered commit makes the write fail instead of being signed as yours
- Vault format upgrades and the commit that follows run under `sync.lock`, so two creds processes
  cannot upgrade or restore a backup at the same time
- A refused remote changes nothing: no ref moves, no file is written, `anchor.json` is untouched
- `join` clones, then unlocks and loads the vault to check it, then records the manifest it saw as
  the state this machine trusts. On failure the clone is removed
- Background sync is a detached `creds sync --quiet` child. It never blocks a command and
  never prompts: it runs with `GIT_TERMINAL_PROMPT=0`, `GIT_SSH_COMMAND=<yours or ssh> -o BatchMode=yes`,
  `SSH_ASKPASS_REQUIRE=never`, `GCM_INTERACTIVE=never`. It needs a live session to merge or to
  take in any remote change, it never asks for the password
- A failed background sync prints `last sync failed: <error>, run creds sync` on later
  commands (not `sync` or `status`) until a sync works. Only the first line of the error is shown
- `sync.lock` is refreshed every 20s and seen as stale after 6m. If the holder pauses for more
  than 3m (laptop sleep), it gives the lock up: its git calls are cancelled and it does not save
  `state.json` or commit. Stale takeover, refresh and release run under an OS file lock on
  `sync.lock.break`, so two processes cannot both take one stale lock. A refresh write that fails
  for a moment is retried, it does not drop the lock. Writes and `passwd` / `recover` hold the lock.
  A pause of 3m inside one refresh can still let two holders overlap (accepted)
- Your remote URL and git credentials are handled by git, not creds

## If your vault may have leaked

Rotate every credential stored inside the vault. Then delete the remote repository and run
`creds init` against a fresh one.

There is no `rekey` command on purpose. Re-encrypting in place cannot reach the copies that already
exist: clones on other machines, forks, the provider's own backups, another device's reflog. Once
the credentials inside are rotated the stolen vault is worthless anyway, so a rekey would buy
nothing while adding a force push and a local directory swap, the most dangerous code path the
project could have.

## Not covered

- Malware or root on your machine
- Hardware attacks, cold boot, swap forensics
- A remote that goes away (keep a local copy or `export --encrypted` backups)
- Forgetting BOTH the password and the recovery code: data is gone
- A remote replay on a machine that has never synced and has no `anchor.json`. It has nothing to
  compare against, so it trusts what it is given on first use. Every later sync is protected

## Terminal output and project files

- Text from git, the remote, entries, import files and project files can hold terminal escapes.
  Example: `ESC]52;c;...BEL` sets the clipboard, `ESC[2J` clears the screen
- Every printed or rendered untrusted string goes through one cleaner (`internal/safetext`).
  C0 controls, DEL, C1 (`0x80-0x9f`) and bad UTF-8 are shown as escaped text.
  Example: ESC prints as the 4 characters `\x1b`.
  Newline and tab stay only in multi line error text. CLI errors, notes, `list`, `get`, `status`,
  the sync banner, prompts and every TUI style go through it
- `Entry.Validate` refuses control characters in path, name and field names.
  Project refs refuse them too
- `status`, setup messages, the TUI settings screen and every git error text hide
  `user:password@` and token query values (`access_token`, `token`, ...) in the remote URL.
  The TUI remote input is masked while you type
- `.creds.toml` is untrusted input: it picks which secrets go to the command.
  creds keeps `trust.json` in `CREDS_HOME` (absolute file path to SHA-256 of the exact bytes read).
  A new or changed file shows its refs and needs a yes. Keys that change how programs run or where
  they connect (`*_PROXY`, `BASH_ENV`, `NODE_OPTIONS`, `LD_*`, `GIT_SSH_COMMAND`, `PATH`, ...) are
  marked risky in that list. No terminal means an error that says
  `run creds trust`. `creds trust [dir]` approves by hand. The hash is of the same bytes that are parsed
- Not covered: a process running as you can edit `trust.json` (same as editing your shell rc)
- Not covered: while a session is live, the command started by `creds run` runs as you and can read
  the session file and the vault, so it can open every entry. Trust picks the env vars, it does not
  guard the vault. `run --lock` ends the session before the command starts. It only helps until the
  next `creds unlock`: a command still running then can read the new session. Docs say to `run` only
  code you trust
- A creds command that reads the session only writes the refreshed time back if the file is still
  the one it read. The check and the write, and `creds lock`, all run under an OS file lock on
  `session.lock`, so `creds lock` in another terminal is never undone
