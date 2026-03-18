# embedrock — Agent Guide

Single-binary Go proxy that translates OpenAI `/v1/embeddings` to Amazon Bedrock.

## Quick Reference

| What | Command |
|------|---------|
| **Build** | `go build ./cmd/embedrock/` |
| **Test** | `go test ./... -v -count=1` |
| **Lint** | `golangci-lint run ./...` |
| **Run** | `./embedrock --port 8089 --region us-east-1` |

## Before You Start

- Read `ARCHITECTURE.md` for the full design and file layout.
- This is a **small codebase** (~6 files). Read it all before making changes.
- Go 1.26+ required. Dependencies: `go mod download`.

## File Layout

```
types.go          → Embedder interface, OpenAI-compatible request/response types
handler.go        → HTTP handler (routing, input parsing, responses)
bedrock.go        → BedrockEmbedder (Titan + Cohere model families)
mock_test.go      → MockEmbedder for tests
handler_test.go   → HTTP handler tests
bedrock_test.go   → Model detection tests
cmd/embedrock/    → CLI entry point (main.go)
```

## Rules

1. **Always run tests before committing:** `go test ./... -v -count=1`
2. **Always run build to verify:** `go build ./cmd/embedrock/`
3. **TDD preferred:** Write a failing test first, then implement.
4. **No new dependencies** without a strong reason — keep the binary small.
5. **Don't break the OpenAI API contract** — clients expect exact `/v1/embeddings` format.
6. **Single-model design:** One embedrock process = one Bedrock model. Per-request model switching is rejected (HTTP 400).
7. **Context propagation:** All `Embed()` calls take `context.Context`. Always pass request context through.
8. **Error hygiene:** Never leak internal/AWS errors to API clients. Return generic messages; log details server-side.
9. **No hardcoded AWS credentials.** Uses instance profile / env vars / shared config.

## Architecture Notes

- `Embedder` interface (`types.go`) is the core abstraction. Handler depends only on this.
- Model family (Titan vs Cohere) is auto-detected by ID prefix in `bedrock.go`.
- Handler parses three input formats: typed single, typed batch, raw JSON fallback.
- Token usage is approximated (~4 chars/token) since we don't have a tokenizer.
- Server binds to `127.0.0.1` by default (uses host IAM creds — don't expose).

## Testing Patterns

- Use `MockEmbedder` from `mock_test.go` — no AWS creds needed for handler tests.
- `httptest.NewRecorder()` + `httptest.NewRequest()` for HTTP tests.
- Bedrock integration tests require real AWS credentials and model access.
- Test both happy path AND error paths (invalid input, embedder failures, model mismatches).

## CI

GitHub Actions (`.github/workflows/`):
- Tests run on push/PR
- Releases build cross-platform binaries via goreleaser
