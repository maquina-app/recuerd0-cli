# Recuerd0 CLI

Go CLI for the Recuerd0 platform. Module: `github.com/maquina/recuerd0-cli`

## Commands

```bash
make build            # Build binary to bin/recuerd0 (version via ldflags)
make test-unit        # Run all unit tests: go test -v ./internal/...
make tidy             # go mod tidy
make clean            # Remove bin/ + dist/ and go clean
make release-check    # goreleaser check (validate .goreleaser.yaml)
make release-snapshot # goreleaser release --snapshot --clean (local dry run, no publish)
```

Go binary: `/opt/homebrew/bin/go` (not in default PATH in some environments).

## Build & Release

**Releases are cut locally from a Mac**, never through GitHub Actions. There is **no release
workflow** in this repo (only `.github/workflows/test.yml` for CI) — do not add one. The
darwin binaries are ad-hoc `codesign`ed in a build post-hook (`scripts/sign-darwin.sh`)
because arm64 SIGKILLs an unsigned Mach-O, and `codesign` only exists on macOS. That is what
pins the release to a Mac.

The GitHub Release is published to this same repo (`maquina-app/recuerd0-cli`).

One release produces: cross-compiled `recuerd0-<os>-<arch>` archives (darwin/linux/windows ×
amd64/arm64; `.tar.gz`, `.zip` for windows), `checksums.txt`, `.deb`/`.rpm` (nfpm), a Homebrew
**cask** pushed to `maquina-app/homebrew-tap` (`Casks/recuerd0.rb` —
`brew install maquina-app/tap/recuerd0`), and `scripts/install.sh` uploaded as a release asset
(backs the `curl | sh` one-liner).

Version is stamped via ldflags `-X main.version={{ .Version }}` (read in
`cmd/recuerd0/main.go`) — it comes from the **tag**, so the tag must exist before goreleaser
runs. A snapshot build reports `X.Y.Z-next` instead.

### Releasing the binary

The gate is local. Run it green before tagging — a tag is public the moment it's pushed:

```bash
make test-unit        # full unit suite
go fmt ./...          # gofmt may realign struct fields; commit the result
make release-check    # goreleaser check — validates .goreleaser.yaml
make release-snapshot # full dry run: builds every target, publishes nothing
```

Then cut it. Working tree must be clean and in sync with `origin/main`:

```bash
# 1. Write the notes FIRST (see below) and commit them — docs/releases/vX.Y.Z.md
# 2. Tag and push; the tag drives the stamped version
git tag vX.Y.Z && git push origin vX.Y.Z

# 3. Build + publish: GitHub Release, archives, checksums, deb/rpm, Homebrew cask
HOMEBREW_TAP_TOKEN=$(gh auth token) GITHUB_TOKEN=$(gh auth token) goreleaser release --clean

# 4. Attach the notes — goreleaser publishes an EMPTY body
gh release edit vX.Y.Z --notes-file docs/releases/vX.Y.Z.md
```

`GITHUB_TOKEN` needs `contents:write` on `maquina-app/recuerd0-cli`; `HOMEBREW_TAP_TOKEN`
needs `contents:write` on `maquina-app/homebrew-tap` (cross-repo — the built-in token can't
write to another repo). A `gh` token with `repo` scope covers both, hence `$(gh auth token)`
for each.

Verify after: `gh release view vX.Y.Z` (12 assets, not draft) and the cask's `version` in
`maquina-app/homebrew-tap`.

### Release notes

Notes are **hand-written prose**, never generated. `changelog: disable: true` in
`.goreleaser.yaml` is deliberate — leave it. Commits here aren't conventional-commit style,
and a generated list restates *what* changed without the *why* that makes notes worth
reading.

Write `docs/releases/vX.Y.Z.md`, commit it, and attach it with the `gh release edit` step
above. The file stays in the repo as the durable record.

Structure (see `docs/releases/v0.6.0.md`):
- `# recuerd0 vX.Y.Z` then `_Released YYYY-MM-DD_`
- `## Features` / `## Fixes`, each entry an `### Imperative headline (#PR)`
- Lead with a short paragraph on the **problem** — the failure or gap that motivated the
  change — then bold-led bullets on what changed
- Close with `## Upgrading`, even when it's just "No config changes required."

Draft from merged PR bodies (`gh pr list --state merged --json number,title,body`), not from
`git log` — the PR descriptions carry the reasoning; commit subjects don't.

## Architecture

Packages: `cmd/recuerd0` (entry), `internal/{errors,response,config,client,commands}`
Config cascade (highest wins): CLI flags > env vars > local `.recuerd0.yaml` > global `~/.config/recuerd0/config.yaml`

## Test Patterns

```go
mock := NewMockClient()
mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: ...}
result := SetTestMode(mock)
SetTestConfig("tok", "https://api.example.com")  // Or SetTestConfigFull for workspace
defer ResetTestMode()
someFlag = "value"
defer func() { someFlag = "" }()
RunTestCommand(func() { someCmd.Run(someCmd, []string{"arg"}) })
// Assert on result.ExitCode, result.Response.Success, mock.GetCalls
```

- `mock_client.go` is in `commands` package (not `_test.go`) — shared across test files
- `RunTestCommand` recovers `testExitSignal` panics from `exitWithError`/`printSuccess`
- `config.SetConfigDir(t.TempDir())` isolates account tests from real config
- `stdinReader` var in `memory.go` allows stdin override — use `io.Reader` type explicitly
- Flag vars are package-level — always reset them in `defer` after setting in tests
- Tests use mutex (`testMu`) — run sequentially per `SetTestMode`/`ResetTestMode` lock

## Integration Testing

Follow `docs/API_TESTING.md` for manual integration tests against the local server (`localhost:3820`). Use `--account local --pretty` for all commands. The doc covers all 13 CLI operations with expected results.

## Code Style

- All output is JSON envelope — no table or human-only formatting
- Every response includes `breadcrumbs` suggesting next CLI commands
- Commands: validate auth/args → `getClient()` → API request → `printSuccess*()` or `exitWithError()`
- Use `response.Breadcrumb` type directly (no aliases)
- Run `go fmt ./...` before committing — gofmt may realign struct fields

## Exit Codes

0=success, 1=general, 2=invalid-args, 3=auth, 4=forbidden, 5=not-found, 6=validation, 7=network, 8=rate-limited
