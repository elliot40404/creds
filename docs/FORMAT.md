# creds file formats

This file shows how creds stores data on disk. It follows the code as it is now.
If code and this file ever disagree, the code wins. Fix this file.

All JSON is UTF-8. Times are RFC 3339 with nanoseconds (Go `time.Time`).
Readers reject unknown JSON keys unless this file says otherwise.

## Layout

Home dir is `~/.config/creds` on every OS. Set `CREDS_HOME` to use another dir.

```
<home>/
  config.toml          user settings, optional
  session              unlocked identity, plaintext, 0600
  state.json           sync state, local only
  trust.json           approved project files, local only
  device.json          vault identity sealed to a plugin key, local only (creds device trust)
  sync.lock            present while a sync runs
  vault/               git repo, the only part that is synced
    .git/
    .gitignore
    vault.json
    identity.pw.age
    identity.recovery.age
    manifest.age
    entries/00.enc .. entries/0f.enc
```

- Dirs are `0700`, files `0600`. On unix creds warns if group or other can read a file. On Windows creds dirs and files get a protected access list: only you and SYSTEM, nothing inherited. creds warns if any other account has access
- Every write is atomic: temp file in same dir, fsync, rename, fsync dir (unix)

## vault/.gitignore

Allowlist. Git only ever sees these paths:

```
*
!.gitignore
!vault.json
!identity.pw.age
!identity.recovery.age
!manifest.age
!entries/
!entries/*.enc
```

The commit guard and `export --encrypted` share one stricter list (`internal/vaultfiles`):
the five fixed files plus
`entries/<2 lowercase hex>.enc`. The guard checks it before each commit. Any `.age` or `.enc`
file must start with `age-encryption.org/v1` or the commit is refused.

## vault/vault.json

```json
{"format_version":3,"recipient":"age1pq1...","created":"2026-09-16T20:45:29.5082464Z"}
```

| key | meaning |
| --- | --- |
| `format_version` | vault format, now `3` |
| `recipient` | age hybrid recipient (`age1pq1...`) of the vault identity |
| `created` | UTC time of `creds init` |

- On load the recipient derived from the unlocked identity must equal `recipient`, else refused
- `format_version` bigger than the program knows: refused. Smaller: needs migration (see Migrations)
- Written last during init, so a half made vault has no `vault.json`

## vault/manifest.age

Signs the whole file set, so a remote cannot serve you an older mix of files that were each
valid once. Without it a remote could replay a bucket from last week next to today's other
buckets and nothing would notice, because every bucket is individually valid.

age file encrypted to the vault recipient. Plaintext is an envelope like a bucket's:

```json
{"mac":"<hex>","manifest":{"format":1,"generation":7,"parents":["<hex>"],"files":[{"path":"...","hash":"<hex>"}]}}
```

| key | meaning |
| --- | --- |
| `format` | manifest format, now `1` |
| `generation` | counts up, starts at `1`, one more than the highest parent |
| `parents` | 0, 1 or 2 manifest IDs, sorted, no repeats. 2 only for a merge |
| `files` | every covered path with the SHA-256 of its exact bytes, sorted by path |

- `files` must list exactly the covered set: `.gitignore`, `vault.json`, `identity.pw.age`,
  `identity.recovery.age` and all 16 buckets. Not `manifest.age`, it cannot hash itself
- MAC is HMAC-SHA-256 over the canonical payload bytes. Its key comes from the identity through
  HKDF with its own info string, `creds manifest mac v1`, so it is not the bucket MAC key
- Manifest ID is the SHA-256 of the canonical payload bytes
- Decoding is strict: unknown members, a non-canonical byte for byte re-encode, an unsorted or
  repeated path or parent, a non lowercase hex hash, generation `0`, or more than 2 parents are
  all refused. Encrypted file capped at 1 MiB
- Written on every commit. A merge writes one with both side manifests as parents

## anchor.json

```json
{"manifest":"<hex>","generation":7}
```

The newest vault state this machine has accepted. Written after a successful commit, sync or
join. Older files also carry a `commit` member, which is read and ignored. A fast forward or a
fresh clone that would adopt a remote state with a lower generation is refused: that is the
replay. A diverged remote may sit below the anchor, since the other machine may have made fewer
commits, but it must be newer than the merge base. A remote at or below the merge base is an old
state replayed on a fake branch and is refused.

Losing `anchor.json` costs the replay protection until the next sync records it again. It never
costs data.

## Identity files

The vault has one age identity: post quantum hybrid (ML-KEM-768 + X25519),
string form `AGE-SECRET-KEY-PQ-1...`. It is stored twice, each wrapped with age scrypt
(passphrase) encryption, work factor logN 18:

| file | passphrase |
| --- | --- |
| `identity.pw.age` | master password, Unicode NFC form |
| `identity.recovery.age` | recovery code, upper case, with dashes |

Plaintext of both files = identity string + `\n`. So stock `age -d` gives a normal age
identity file. See [RECOVERY.md](RECOVERY.md).

Both passphrases are turned into Unicode NFC form (`golang.org/x/text/unicode/norm`) before
wrap and unwrap. Example: `�` typed as one char (NFC) or as `e` + accent (NFD, some macOS
input) gives the same key. ASCII is not changed, so the recovery code is not affected.
Unwrap tries the NFC form first, then the raw bytes if they differ, so a file wrapped
before this rule still opens. Stock age uses the raw bytes: type the NFC form there.

Recovery code: 30 base32 chars (A-Z, 2-7, 150 bits), groups of 5 joined with `-`.
Example `NGIO5-6NIEE-YSA3C-PPSTE-43F2C-VBKNE`. creds accepts it typed in any case, with
spaces or without dashes, and turns it back into the dashed upper case form before use.
Stock age needs the exact dashed upper case form.

`passwd` rewrites only `identity.pw.age`. `recover` asks the recovery code, then sets a new
password the same way.

## Buckets: vault/entries/NN.enc

16 files, `00.enc` to `0f.enc` (two lower case hex digits). All 16 always exist, empty
ones too.

Entry goes to slot `SHA-256(16 raw uuid bytes)[0] % 16`. Not the uuid byte itself,
because uuid v7 starts with a timestamp.

### Layers, outside in

1. age file, encrypted to the vault recipient (`vault.json` `recipient`)
2. padded plaintext: JSON envelope, then ASCII spaces (`0x20`) up to the pad size.
   Pad size = 4096, doubled until it is at least the JSON length.
   Example: 300 byte JSON -> 4096 bytes. 5000 byte JSON -> 8192 bytes
3. envelope:

```json
{"mac":"<64 hex chars>","bucket":{"format":1,"slot":8,"entries":[...]}}
```

4. `bucket` object:

| key | meaning |
| --- | --- |
| `format` | bucket format, now `1` |
| `slot` | slot number as JSON int (8, not `"08"`) |
| `entries` | list of entries, sorted by path, `[]` when empty |

JSON decoders ignore the trailing spaces, so `age -d ... | jq` works.

### MAC

- key = HKDF-SHA256(secret = identity string `AGE-SECRET-KEY-PQ-1...`, salt empty,
  info `creds bucket mac v1`, 32 bytes)
- `mac` = lower case hex of HMAC-SHA256(key, exact bytes of the `bucket` value as written)

Why: anyone with the public recipient can make a valid age file. The MAC proves the
bucket was written by someone holding the identity.

### Checks on load, in order

1. file at most 64 MiB, starts with `age-encryption.org/v1`
2. age decrypt with the identity
3. envelope parse, MAC check
4. `bucket` strict parse, `format == 1`, `slot` equals file slot
5. each entry: slot of its id equals file slot, entry valid
6. across all buckets: no duplicate path, no duplicate id

Any failure stops the load. Save rewrites only changed buckets.

## Entry

```json
{
  "id": "01a0abf8-20a0-7ee7-bf58-0ea18e8cd253",
  "path": "work/github",
  "name": "GitHub",
  "type": "login",
  "username": "octocat",
  "host": "",
  "url": "https://github.com",
  "notes": "",
  "tags": ["work"],
  "fields": [{"name": "password", "value": "hunter2", "secret": true}],
  "params": {"sslmode": "require"},
  "created": "2026-09-17T02:16:00.3529314+05:30",
  "updated": "2026-09-17T02:16:00.3529314+05:30",
  "machine": "laptop",
  "history": [
    {
      "machine": "desktop",
      "at": "2026-09-16T11:02:00.0000000+05:30",
      "username": "octocat",
      "fields": [{"name": "password", "value": "hunter1", "secret": true}]
    }
  ]
}
```

| key | rule |
| --- | --- |
| `id` | uuid v7, set on first save, never changes |
| `path` | required, unique, `/` groups things (`work/db/prod`) |
| `type` | one of `login api database ssh note command env generic` |
| `name username host url notes tags fields params` | left out when empty |
| `fields[].name` | required, not empty |
| `fields[].secret` | left out when false |
| `created` `updated` | local time with offset |
| `machine` | who made this edit, `vault.machine` or the hostname. Left out when empty |
| `history` | previous versions, newest first, at most `vault.history` of them. Left out when empty |

### history

A revision holds the value fields it supersedes (`name username host url notes tags fields
params`), the machine that wrote them and `at`, the `updated` stamp they had. It carries no
`id`, `path`, `type` or `created`: those are the entry's, not the version's.

`vault.Put` pushes the pre image on every write that changes a value, then cuts the list to
`vault.history` (default 3). Writing with `vault.history = 0` clears the list. A write that
changes nothing pushes nothing.

A three way merge picks one side's entry as it always did, `history` takes part in neither
that choice nor the conflict test. The merged entry then gets the union of both sides'
revisions, deduped on `at` plus values, newest first, cut to the longer of the two lists. The
union is symmetric, so two machines merging the same pair land on the same list.

**This keeps old secret values in the vault.** A rotated password stays decryptable for
`vault.history` more edits of that entry. Set `vault.history = 0` to turn it off.

Secret values live in `fields`. `database` entries: `host` and `username` on the entry,
fields `engine`, `scheme`, `port`, `database`, `password` (secret), query options in `params`.
`env` entries: each field is one env var. `command` entries are text only, never run.

## Session: \<home\>/session

```json
{"identity":"AGE-SECRET-KEY-PQ-1...","created":"...","last_used":"..."}
```

- Plaintext unlocked identity. File mode `0600`. Never in git
- Valid while `now - last_used < session.idle` and `now - created < session.hard`
- Each use rewrites `last_used`
- Expired, broken, or `last_used` in the future: file is deleted, user is asked again
- `creds lock` deletes it

## Sync state: \<home\>/state.json

```json
{
  "last_pull": "...",
  "last_sync": "...",
  "last_result": "pushed",
  "last_error": "",
  "conflicts": [{"file": "entries/03.enc", "paths": ["work/db"], "ids": ["..."], "choice": 1}],
  "password_changed": "...",
  "clear_error": ""
}
```

| key | meaning |
| --- | --- |
| `last_pull` | UTC time of last good fetch |
| `last_sync` | UTC time of last try, good or failed. Reads start a background sync when this is older than `sync.stale` |
| `last_result` | `up-to-date`, `pushed`, `fast-forwarded`, `merged`, `conflict`, `error`, `needs unlock` (remote changed any vault file but no live session, so the tree was left as is; `last_pull` is not moved) |
| `last_error` | error text of last failed try. Cleared by the next try. While `last_result` is `error` and a remote is set, commands show `last sync failed: <first line>, run creds sync`. `status --json` shows it as `last_error` (plain message) and `last_error_raw` (stored text as is) |
| `conflicts[].file` | vault file in conflict |
| `conflicts[].paths` `ids` | entries in conflict |
| `conflicts[].choice` | missing = not picked, `1` = mine, `2` = theirs (set by `creds resolve`) |
| `password_changed` | UTC time a sync pulled a different `identity.pw.age`. Commands warn until you unlock with the password or run `passwd`, which clears it |
| `clear_error` | first line of the error when the background clipboard clear failed. The next command warns once and clears it |

`state.json.lock` next to it is an OS file lock held for each read, change and write of the file.

Empty keys are left out. Missing file = empty state.

## Trust file: \<home\>/trust.json

Which project `.creds.toml` files you approved. Local only, never synced.

```json
{"version":1,"projects":{"D:\\code\\app\\.creds.toml":"<64 hex chars>"}}
```

| key | meaning |
| --- | --- |
| `version` | trust file format, now `1`. Any other value is an error |
| `projects` | absolute file path to lower case hex SHA-256 of the exact file bytes that were parsed |

- Trusted = the file is a key AND its hash still matches. Change one byte and creds asks again
- Written by `creds trust` and by a yes at the prompt. Max size 1 MiB, unknown keys are an error
- Holds no secrets, only paths. Missing file = nothing trusted

## Device trust: \<home\>/device.json

Written by `creds device trust`. Lets this machine unlock with an age plugin key (Touch ID via
`age-plugin-se`, a YubiKey via `age-plugin-yubikey`, a TPM via `age-plugin-tpm`) instead of the
master password. Local only, never in `vault/`, never synced.

```json
{"plugin":"se","identity":"AGE-PLUGIN-SE-1...","recipient":"age1se1...","vault":"age1pq1...","trusted":"...","confirmed":"...","sealed":"<base64 age file>"}
```

| key | meaning |
| --- | --- |
| `plugin` | plugin name, creds runs `age-plugin-<plugin>` found on PATH |
| `identity` | plugin identity string. For hardware plugins a handle, the private key stays in the hardware |
| `recipient` | plugin recipient `sealed` was made for. Empty: the identity itself was used as recipient |
| `vault` | vault recipient from `vault.json` at trust time |
| `trusted` | when `creds device trust` ran |
| `confirmed` | last unlock with the master password |
| `sealed` | age file, recipient `recipient`, plaintext the vault identity string |

- Unlock order: live session, then this file (the plugin asks for a touch or PIN), then the master password
- `vault` must equal the recipient in `vault.json`, and the opened identity must match it. If not,
  the vault key changed: the file is deleted with a warning
- `now - confirmed >= device.max_age` (default `72h`, `0` = never): the file is skipped and the
  master password is asked. That unlock moves `confirmed` to now
- A cancel, a missing plugin or any plugin error: one warning, then the master password. The file stays
- Trust seals the identity, then opens it once (the first touch) before the file is written, so a
  broken key pair is never saved
- Strict decode, max size 64 KiB, unknown keys or bad fields: file deleted. Readable by others:
  file deleted, like the session
- `creds device untrust` deletes it. `creds passwd` and `creds recover` offer to delete it

## Sync lock: \<home\>/sync.lock

```json
{"pid":1234,"time":"2026-09-17T02:16:00Z"}
```

Made with exclusive create. Stale when `time` is more than 6 minutes away from now
(past or future), or the pid is gone. Taking a stale lock, refresh and release all run while
holding an OS file lock (`LockFileEx` / `flock`) on `sync.lock.break`, and check the lock again
first, so only one process can take a stale lock and nobody removes a lock that is not theirs.
Removed when the holder ends. The holder rewrites `time` every 20 seconds. A failed rewrite
(for example a reader has the file open on Windows) is tried again on the next tick. A holder
that could not rewrite for 3 minutes (for example the machine slept), or finds the lock is not
its own, gives up: it stops its work and does not save `state.json` or commit.
`creds sync`, every write (add, edit, rm and the like) and `passwd` / `recover` wait for the
lock up to 2 minutes.
`creds sync --quiet` skips at once when the lock is held. Reads do not start a background
sync while the lock is held.

## Git

- Branch `master`, remote `origin`
- Author and committer come from `sync.name`/`sync.email`, else the global git identity, else `creds <creds@localhost>`. Message `update vault`, or
  `Merge remote changes` for a sync merge. No entry names or paths in git metadata
- Git runs with hooks off, no signing, `core.autocrlf=false`, `core.symlinks=false`,
  `core.fsmonitor=false`, `transfer.fsckObjects=true`, `GIT_TERMINAL_PROMPT=0`.
  Protocols: `protocol.allow=never` plus `https`, `ssh` and `file` set to `always`
- `join` uses `git clone`, not init + remote add
- Background sync (`creds sync --quiet`, detached) also sets `GIT_SSH_COMMAND=<yours or ssh> -o BatchMode=yes`,
  `SSH_ASKPASS_REQUIRE=never`, `GCM_INTERACTIVE=never`. It starts after a command that made a
  commit, and on reads when `last_sync` is older than `sync.stale`. It merges only with a live
  session. Exit code is always 0, the result goes to `state.json`

## config.toml

All keys optional. Durations use Go syntax (`90s`, `15m`, `4h`), minimum `1s` (`device.max_age` may be `0`).
Unknown keys are an error. Missing file = all defaults.

The key list lives in one place, `internal/config/keys.go` (`config.Fields`). `creds config`,
`creds config get|set|unset` and the TUI settings screen all read it, so a new key shows up
everywhere at once. Do not copy the list into other code.

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

[device]
max_age = "72h"

[render.formats]
"postgres.url" = "postgres://{{.username}}@{{.host}}:{{.port}}/{{.database}}"
"redis.local" = "redis-cli -h {{sh .host}}"
```

| key | default | rule |
| --- | --- | --- |
| `session.idle` | `15m` | not more than `session.hard` |
| `session.hard` | `4h` | |
| `clipboard.clear` | `30s` | |
| `sync.stale` | `5m` | a read starts a background sync when the last sync try is older than this |
| `device.max_age` | `72h` | a trusted device asks the master password again after this long, `0` never asks, max `720h` |
| `render.shell` | `pwsh` on Windows, else `bash` | quoting style for command formats, `bash` or `pwsh` |
| `render.formats` | empty | key `engine.format`, value Go template, checked at load |

Engines: `postgres`, `redis`, `mongo`. Template data: `.scheme .host .port .username .password .database .params .url`.
Helpers: `sh` (quote for `render.shell`), `envset`, `envline`, `envunset`, `flag`, `pathesc`, `queryesc`.
Same key as a built in format replaces it, a new name adds one.

`creds config set` and the TUI validate the whole config before it is written, so a bad value
never lands in the file. Only `render.formats.*` keys can be removed.

## Project file: .creds.toml

In a project dir, used by `creds env` and `creds run` when no path is given.

```toml
env = "work/api-env"

[map]
DATABASE_URL = "work/db|url"
API_TOKEN = "work/api:token"
```

- `env`: path of an `env` type entry, each field becomes a var
- `map`: var name to ref. `path:field` = one field value, `path|format` = rendered
  database format. The ref splits on the last `:` or `|`
- At least one of `env` or `map` is required

## Export files

### Plain JSON (`export --plain --format json`)

```json
{"format":"creds-export","version":1,"entries":[<entry>, ...]}
```

Entries use the Entry format above, secrets in clear text. `creds import` reads only this
format (max 64 MiB). Imported entries get new ids. With `--overwrite` a same path entry
keeps its old id and created time.

### Plain CSV (`export --plain --format csv`)

Header `path,type,username,host,url,field,value,secret`. One row per field. Entry without
fields = one row with empty field cells. `secret` is `true` or `false`. Notes, tags and
params are not exported. Not importable.

Spreadsheet safety: a cell that starts with `=`, `+`, `-`, `@`, tab or CR gets a `'` in
front, so a spreadsheet shows it as text and never runs it as a formula. Example: value
`=HYPERLINK("x")` is written as `'=HYPERLINK("x")`, and `-abc` as `'-abc`. Remove the
leading `'` if you read the file with a script.

### Encrypted tar (`export --encrypted`)

PAX tar of the synced vault files only, in this order:

```
.gitignore
vault.json
identity.pw.age
identity.recovery.age
entries/00.enc .. entries/0f.enc
```

Files that are missing are skipped. Mode `0600`. No unlock needed, nothing decrypted.
No restore command: untar into `<home>/vault/` by hand.

## Migrations

- Version lives in `vault.json` `format_version`
- Upgrades run in order, one per version (`0 -> 1`, ...). Each upgrade never deletes old data
- Before a file is changed, its old copy goes to `vault/.migrate-backup/`. On success the
  backup dir is removed. On failure, or if a backup dir exists at start, files are put back
- Version 0: `{"recipient": "...", "created_at": "..."}` with no `format_version`.
  Upgrade renames `created_at` to `created` and adds `format_version: 1`
- Version 1 to 2: version bump only
- Version 2 to 3: version bump plus a `.gitignore` that lets `manifest.age` through. The manifest
  itself is written on the commit that follows the upgrade, because it needs the unlocked
  identity. A v1 or v2 binary refuses a v3 vault outright, it does not guess
- Upgrades run automatically on unlock. The backup dir plus the rollback covers a failure, and a
  failed upgrade leaves a vault the old binary still opens
- Version 1: same `vault.json` as version 2. Entries gained `machine` and `history`, both
  optional, so buckets need no rewrite. Upgrade only sets `format_version: 2`

The bump exists for the other direction. Bucket JSON is parsed strictly, so a creds that
predates `history` would fail with an unknown member error on the first v2 entry it meets.
The version check runs before that parse, so it says `vault format is newer than this
program` with the hint to install the latest release instead.
