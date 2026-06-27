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

Releases are cut **manually and locally from a Mac** with [GoReleaser](https://goreleaser.com) (`.goreleaser.yaml`) — there is **no release CI workflow** (only `.github/workflows/test.yml` for CI). The GitHub Release is published to this same repo (`maquina-app/recuerd0-cli`).

One release produces: cross-compiled `recuerd0-<os>-<arch>` archives (darwin/linux/windows × amd64/arm64; `.tar.gz`, `.zip` for windows), `checksums.txt`, `.deb`/`.rpm` (nfpm), a Homebrew **cask** pushed to `maquina-app/homebrew-tap` (`Casks/recuerd0.rb` — `brew install maquina-app/tap/recuerd0`), and `scripts/install.sh` uploaded as a release asset (backs the `curl | sh` one-liner).

- Version is stamped via ldflags `-X main.version={{ .Version }}` (read in `cmd/recuerd0/main.go`).
- macOS binaries are ad-hoc codesigned in a build post-hook (`scripts/sign-darwin.sh`) so arm64 won't SIGKILL them — **release must be cut from a Mac**.

Cutting a release:
```bash
git tag vX.Y.Z && git push origin vX.Y.Z
HOMEBREW_TAP_TOKEN=$(gh auth token) GITHUB_TOKEN=$(gh auth token) goreleaser release --clean
```
`GITHUB_TOKEN` needs `contents:write` on `maquina-app/recuerd0-cli`; `HOMEBREW_TAP_TOKEN` needs `contents:write` on `maquina-app/homebrew-tap` (cross-repo). A `gh` token with `repo` scope covers both. Validate/dry-run first with `make release-check` / `make release-snapshot`.

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
