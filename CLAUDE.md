# Recuerd0 CLI

Go CLI client for the Recuerd0 platform. Module: `github.com/maquina/recuerd0-cli`

## Commands

```bash
make build        # Build binary to bin/recuerd0 (version via ldflags)
make test-unit    # Run all unit tests: go test -v ./internal/...
make tidy         # go mod tidy
make clean        # Remove bin/ and go clean
```

Go is at `/opt/homebrew/bin/go` (not in default PATH in some environments).

## Architecture

```
cmd/recuerd0/main.go              # Entry point, version injection via -ldflags
internal/
  errors/errors.go                # CLIError with Code, Message, Status, ExitCode
  response/response.go            # JSON envelope: success, data, breadcrumbs, pagination, summary, meta
  config/config.go                # Multi-account config, cascading resolution
  client/interface.go             # API interface (Get, Post, Patch, Delete, GetWithPagination)
  client/client.go                # HTTP implementation with Bearer auth, Link header pagination
  commands/root.go                # Root command, global flags, PersistentPreRun, test infrastructure
  commands/mock_client.go         # Mock client (not _test.go — shared across test files)
  commands/{account,workspace,memory,search,version}.go  # Command implementations
```

Config cascade (highest wins): CLI flags > env vars > local `.recuerd0.yaml` > global `~/.config/recuerd0/config.yaml`

## Test Patterns

Every command test follows this pattern:

```go
mock := NewMockClient()
mock.GetResponse = &client.APIResponse{StatusCode: 200, Data: ...}

result := SetTestMode(mock)       // Locks mutex, sets clientFactory
SetTestConfig("tok", "https://api.example.com")  // Or SetTestConfigFull for workspace
defer ResetTestMode()             // Unlocks mutex, clears state

// Reset any flag vars used by the command
someFlag = "value"
defer func() { someFlag = "" }()

RunTestCommand(func() {
    someCmd.Run(someCmd, []string{"arg"})
})

// Assert on result.ExitCode, result.Response.Success, mock.GetCalls, etc.
```

- `mock_client.go` is in the `commands` package (not a `_test.go` file) so all test files can use it
- `RunTestCommand` recovers `testExitSignal` panics from `exitWithError`/`printSuccess`
- `config.SetConfigDir(t.TempDir())` isolates account tests from real config
- `stdinReader` var in `memory.go` allows stdin override in tests — use `io.Reader` type explicitly

## Code Style

- All output is JSON envelope — no table or human-only formatting
- Every response includes `breadcrumbs` suggesting next CLI commands
- Commands: validate auth/args → call `getClient()` → API request → `printSuccess*()` or `exitWithError()`
- Use `response.Breadcrumb` type directly (no aliases)
- Run `go fmt ./...` before committing — gofmt may realign struct fields

## Exit Codes

0=success, 1=general, 2=invalid-args, 3=auth, 4=forbidden, 5=not-found, 6=validation, 7=network, 8=rate-limited

## Gotchas

- Go binary path: use `/opt/homebrew/bin/go` if `go` is not found
- Tests use a mutex (`testMu`) — tests within `commands` package run sequentially per `SetTestMode`/`ResetTestMode` lock
- Flag vars are package-level — always reset them in `defer` after setting in tests
- `--content -` reads from stdin; override `stdinReader` in tests with `func() io.Reader`
