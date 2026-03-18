# embedrock — Agent Guide

Single-binary Go proxy that translates OpenAI `/v1/embeddings` to Amazon Bedrock.

## Quick Reference

| What | Command |
|------|---------|
| **Build** | `go build ./cmd/embedrock/` |
| **Test** | `go test ./... -v -count=1` |
| **Lint** | `golangci-lint run ./...` |
| **Run** | `./embedrock --port 8089 --region us-east-1` |
| **Update** | `./embedrock update` |
| **Install daemon** | `sudo ./embedrock install-daemon` |

## Before You Start

- Read `ARCHITECTURE.md` for the full design and file layout.
- This is a **small codebase** (~10 files). Read it all before making changes.
- Go 1.26+ required. Dependencies: `go mod download`.

## File Layout

```
types.go                    → Embedder interface, OpenAI-compatible request/response types
handler.go                  → HTTP handler (routing, input parsing, responses)
bedrock.go                  → BedrockEmbedder (Titan + Cohere model families)
mock_test.go                → MockEmbedder for tests
handler_test.go             → HTTP handler tests (20 tests)
bedrock_test.go             → Model detection tests
cmd/embedrock/main.go       → CLI entry point (flags, subcommands, server startup)
cmd/embedrock/update.go     → Self-update command (GitHub releases, checksum verification)
cmd/embedrock/update_test.go → Update tests (mock GitHub API, 6 tests)
cmd/embedrock/daemon.go     → Install-daemon command (systemd unit file generation)
cmd/embedrock/daemon_test.go → Daemon tests (unit file generation, root check)
```

## Rules

1. **Always run tests before committing:** `go test ./... -v -count=1`
2. **Always run build to verify:** `go build ./cmd/embedrock/`
3. **TDD preferred:** Write a failing test first, then implement.
4. **No new dependencies** without a strong reason — keep the binary small (stdlib only in cmd/).
5. **Don't break the OpenAI API contract** — clients expect exact `/v1/embeddings` format.
6. **Single-model design:** One embedrock process = one Bedrock model. Per-request model switching is rejected (HTTP 400).
7. **Context propagation:** All `Embed()` calls take `context.Context`. Always pass request context through.
8. **Error hygiene:** Never leak internal/AWS errors to API clients. Return generic `"embedding failed"` message; log details server-side.
9. **No hardcoded AWS credentials.** Uses instance profile / env vars / shared config.
10. **HTTP timeouts required:** All HTTP clients must have explicit timeouts. Server uses `http.Server` with read/write/idle timeouts.
11. **Test stability:** Tests must NOT modify the real test binary. Use `t.TempDir()` for temp files. Run tests twice to verify stability.

## Architecture Notes

- `Embedder` interface (`types.go`) is the core abstraction. Handler depends only on this.
- Model family (Titan vs Cohere) is auto-detected by ID prefix in `bedrock.go`.
- Handler parses three input formats: typed single, typed batch, raw JSON fallback.
- Token usage is approximated (~4 chars/token) since we don't have a tokenizer.
- Server binds to `127.0.0.1` by default (uses host IAM creds — don't expose).
- `runUpdate` / `runUpdateTo` split allows tests to inject a temp binary path.
- `updateHTTPClient` has 30s timeout — never use `http.DefaultClient` or bare `http.Get`.

## CLI Structure

```
main.go:
  1. Parse flags (--port, --host, --region, --model, --version)
  2. Handle --version (before subcommands)
  3. Handle subcommands: "update", "install-daemon", or unknown → error
  4. Start server
```

Subcommands are positional args after flags: `embedrock --port 9090 install-daemon`

## Testing Patterns

- Use `MockEmbedder` from `mock_test.go` — no AWS creds needed for handler tests.
- `httptest.NewRecorder()` + `httptest.NewRequest()` for HTTP tests.
- `httptest.NewServer` for mocking GitHub API in update tests.
- `t.TempDir()` for temp files that auto-clean.
- `runUpdateTo(version, apiBase, execPathOverride)` for update tests — never use `runUpdate` directly in tests (would overwrite the test binary).
- Bedrock integration tests require real AWS credentials and model access.
- Test both happy path AND error paths (invalid input, embedder failures, model mismatches, checksum mismatches, API errors).

## CI

GitHub Actions (`.github/workflows/`):
- `ci.yml` — Tests run on push/PR
- `release.yml` — Builds cross-platform binaries on tag push (linux/darwin × arm64/amd64)
- `checksums.txt` included in every release for `embedrock update` verification
