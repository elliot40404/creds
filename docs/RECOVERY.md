# Manual recovery with stock age

Use this when creds is gone or broken and you need your secrets back.
You only need the `age` tool and one of: master password or recovery code.
These steps were run for real against a vault made by creds (age v1.3.2, Windows) and
checked again against a vault made by the current build.

## 1. Get age

Need age v1.3.0 or newer (older age does not know post quantum `AGE-SECRET-KEY-PQ-1` keys).

- Download: https://github.com/FiloSottile/age/releases
- Or with Go: `go install filippo.io/age/cmd/age@v1.3.2`
- Or package manager: `brew install age`, `apt install age`, `winget install FiloSottile.age`
  (check `age --version` is at least v1.3.0)

## 2. Find the vault files

Any copy works: the local vault, a git clone of your remote, or an untarred
`creds export --encrypted` file.

```
vault/
  identity.pw.age          open with master password
  identity.recovery.age    open with recovery code
  entries/00.enc .. 0f.enc your entries
```

Local vault is `~/.config/creds/vault` (or `$CREDS_HOME/vault`).
If creds still runs, `creds paths` prints the exact spot.
From the remote: `git clone <your-remote-url> vault`.

## 3. Get the identity

With the master password:

```sh
age -d -o identity.txt vault/identity.pw.age
Enter passphrase:        <- type the master password
```

age asks for the passphrase on the terminal itself. You cannot pipe it in
(`echo pw | age -d ...` just waits), so run these steps by hand in a real terminal.

creds stores the password in Unicode NFC form. Stock age uses the bytes you type.
A plain ASCII password is fine. With accents or other non ASCII letters, make sure the
terminal sends NFC (example: `�` as one char, not `e` + accent). If age says
`incorrect passphrase` for the right password, this is the likely cause.

Or with the recovery code:

```sh
age -d -o identity.txt vault/identity.recovery.age
Enter passphrase:        <- type the code EXACTLY as shown at init
```

The recovery code must be typed upper case with dashes, like
`NGIO5-6NIEE-YSA3C-PPSTE-43F2C-VBKNE`. Without dashes age says
`incorrect passphrase` (creds itself accepts both, stock age does not).

`identity.txt` now holds one line `AGE-SECRET-KEY-PQ-1...`. This key opens everything.
Keep it safe and delete it when done.

Tip: after `passwd`, old git commits still hold the old `identity.pw.age`. The old
password opens those old copies.

## 4. Decrypt entries

One bucket:

```sh
age -d -i identity.txt vault/entries/08.enc
```

Output is JSON then many spaces (padding):

```json
{"mac":"3081...","bucket":{"format":1,"slot":8,"entries":[{"id":"01a0abf8-...","path":"work/github","type":"login","username":"octocat","url":"https://github.com","fields":[{"name":"password","value":"hunter2-demo-secret","secret":true}],"created":"...","updated":"..."}]}}
```

Secrets are in `entries[].fields[].value`. Layout is in [FORMAT.md](FORMAT.md).

All buckets at once, bash:

```sh
for f in vault/entries/*.enc; do age -d -i identity.txt "$f"; echo; done > all.json
```

PowerShell:

```powershell
Get-ChildItem vault\entries\*.enc | ForEach-Object { age -d -i identity.txt $_.FullName; "" } > all.json
```

Find one entry, with jq:

```sh
for f in vault/entries/*.enc; do age -d -i identity.txt "$f"; done \
  | jq -c '.bucket.entries[] | select(.path == "work/github")'
```

Without jq, just search the text for the path, for example `grep work/github all.json`.

## 5. Clean up

`all.json` and `identity.txt` are plain secrets. Delete them.
If you think the identity leaked, make a new vault and move the secrets over:
`creds export --plain` from the old vault, `creds init` + `creds import` in a new home.

## What age alone does not check

creds also checks a MAC inside each bucket (see FORMAT.md). age cannot. If someone
could write to your git remote, the decrypted data might be old or fake but still
readable. For a trusted copy, use your own local vault or `creds` itself.

## With creds still working

- Forgot password, have code: `creds recover`
- Have password: `creds unlock`, then `creds export --plain -o backup.json`
- New machine: `creds join <remote-url>`
