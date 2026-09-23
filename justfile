set windows-shell := ["pwsh", "-NoLogo", "-NoProfile", "-Command"]

bin := if os_family() == "windows" { "bin/creds.exe" } else { "bin/creds" }

default: verify

build:
    go build -trimpath -o {{bin}} ./cmd/creds

install:
    go install -trimpath ./cmd/creds

hooks:
    git config core.hooksPath .githooks

[unix]
uninstall:
    #!/usr/bin/env sh
    dir="$(go env GOBIN)"
    [ -n "$dir" ] || dir="$(go env GOPATH)/bin"
    rm -f "$dir/creds"

[windows]
uninstall:
    $d = go env GOBIN; if (-not $d) { $d = Join-Path (go env GOPATH) "bin" }; Remove-Item -Force -ErrorAction Ignore (Join-Path $d "creds.exe")

fmt:
    golangci-lint fmt ./...

fmt-check:
    golangci-lint fmt --diff ./...

fix-check:
    go fix -diff ./...

vet:
    go vet ./...

lint:
    golangci-lint run ./...

vuln:
    govulncheck ./...

mod-check:
    go mod verify
    go mod tidy -diff

test:
    go test -race ./...

verify: fmt-check fix-check vet lint vuln mod-check test

linux-image := "golang:1.27"
no-docker := "docker not found, skipping test-linux"
linux-test := 'docker run --rm -v "' + justfile_directory() + ':/src:ro" -w /w ' + linux-image + ' sh -c "cp -r /src /w && cd /w/src && go test -race ./..."'

[unix]
test-linux:
    #!/usr/bin/env sh
    if ! command -v docker >/dev/null 2>&1; then
        echo "{{no-docker}}"
        exit 0
    fi
    {{linux-test}}

[windows]
test-linux:
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { Write-Host "{{no-docker}}"; exit 0 }; {{linux-test}}; exit $LASTEXITCODE

fuzztime := "10s"

fuzz:
    go test -run='^$' -fuzz='^FuzzParse$' -fuzztime={{fuzztime}} ./internal/envfile
    go test -run='^$' -fuzz='^FuzzWrite$' -fuzztime={{fuzztime}} ./internal/envfile
    go test -run='^$' -fuzz='^FuzzParseRef$' -fuzztime={{fuzztime}} ./internal/envfile
    go test -run='^$' -fuzz='^FuzzLoadProject$' -fuzztime={{fuzztime}} ./internal/envfile
    go test -run='^$' -fuzz='^FuzzRoundtrip$' -fuzztime={{fuzztime}} ./internal/envfile
    go test -run='^$' -fuzz='^FuzzLoadTrust$' -fuzztime={{fuzztime}} ./internal/envfile
    go test -run='^$' -fuzz='^FuzzTrustedSum$' -fuzztime={{fuzztime}} ./internal/envfile
    go test -run='^$' -fuzz='^FuzzOpenBucketPlaintext$' -fuzztime={{fuzztime}} ./internal/vault
    go test -run='^$' -fuzz='^FuzzOpenBucketJSON$' -fuzztime={{fuzztime}} ./internal/vault
    go test -run='^$' -fuzz='^FuzzValidatePath$' -fuzztime={{fuzztime}} ./internal/vault
    go test -run='^$' -fuzz='^FuzzEntryValidate$' -fuzztime={{fuzztime}} ./internal/vault
    go test -run='^$' -fuzz='^FuzzLoad$' -fuzztime={{fuzztime}} ./internal/session
    go test -run='^$' -fuzz='^FuzzLoadMeta$' -fuzztime={{fuzztime}} ./internal/format
    go test -run='^$' -fuzz='^FuzzNormalizeRecoveryCode$' -fuzztime={{fuzztime}} ./internal/crypto
    go test -run='^$' -fuzz='^FuzzUnwrapIdentity$' -fuzztime={{fuzztime}} ./internal/crypto
    go test -run='^$' -fuzz='^FuzzParseRender$' -fuzztime={{fuzztime}} ./internal/render
    go test -run='^$' -fuzz='^FuzzDecodeImport$' -fuzztime={{fuzztime}} ./internal/app
    go test -run='^$' -fuzz='^FuzzLoad$' -fuzztime={{fuzztime}} ./internal/config
    go test -run='^$' -fuzz='^FuzzSave$' -fuzztime={{fuzztime}} ./internal/config
    go test -run='^$' -fuzz='^FuzzCheckRepoName$' -fuzztime={{fuzztime}} ./internal/gh
    go test -run='^$' -fuzz='^FuzzRepos$' -fuzztime={{fuzztime}} ./internal/gh
    go test -run='^$' -fuzz='^FuzzURLAndNotFound$' -fuzztime={{fuzztime}} ./internal/gh
    go test -run='^$' -fuzz='^FuzzGhError$' -fuzztime={{fuzztime}} ./internal/gh
    go test -run='^$' -fuzz='^FuzzText$' -fuzztime={{fuzztime}} ./internal/safetext
