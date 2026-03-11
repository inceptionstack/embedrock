# Architecture

## Overview

embedrock is an OpenAI-compatible embedding proxy for Amazon Bedrock. It translates standard `/v1/embeddings` API calls into Bedrock `InvokeModel` requests, allowing any tool that speaks the OpenAI embeddings API to use Bedrock models with zero code changes.

```
┌─────────────┐     POST /v1/embeddings      ┌─────────────┐     InvokeModel      ┌─────────────┐
│   Client     │ ──────────────────────────►  │  embedrock  │ ──────────────────►  │   Bedrock   │
│  (OpenClaw,  │  { input: "text",            │  (Go binary) │  Titan or Cohere    │   Runtime   │
│  LangChain)  │    model: "..." }            │  port 8089   │  format (auto)      │             │
│              │ ◄──────────────────────────  │              │ ◄──────────────────  │             │
│              │  { data: [{ embedding }] }   │              │  model response     │             │
└─────────────┘                               └─────────────┘                     └─────────────┘
```

## File Structure

```
embedrock/
├── cmd/embedrock/main.go   # CLI entry point (flags, server startup)
├── types.go                # Embedder interface, OpenAI types, errors
├── bedrock.go              # BedrockEmbedder (Titan + Cohere formats)
├── handler.go              # HTTP handler (routing, parsing, responses)
├── mock_test.go            # MockEmbedder (test helper)
├── bedrock_test.go         # Model detection tests
├── handler_test.go         # HTTP handler tests (16 tests)
├── go.mod / go.sum         # Dependencies
├── .github/workflows/      # CI + release automation
├── README.md               # User docs
└── ARCHITECTURE.md         # This file
```

## Components

### `types.go` — Contracts

The shared type layer. No logic, just shapes.

- **`Embedder` interface** — `Embed(text string) ([]float64, error)`. The only abstraction boundary. Everything depends on this.
- **Request types** — `EmbeddingRequest` (single) and `EmbeddingRequestBatch` (array), matching OpenAI's API.
- **Response types** — `EmbeddingResponse`, `EmbeddingData`, `Usage` — exact OpenAI format.
- **`EmbedError`** — typed error for embedding failures.

### `bedrock.go` — Bedrock Embedder

The real `Embedder` implementation. Handles two model families:

- **Titan** — `{"inputText": "..."}` → `{"embedding": [...]}`
- **Cohere v3** — `{"texts": [...], "input_type": "search_query"}` → `{"embeddings": [[...]]}`
- **Cohere v4** — same request, different response: `{"embeddings": {"float": [[...]]}}`

Model family is detected by ID prefix (`cohere.` vs everything else). The shared `invoke()` method handles the Bedrock SDK call.

### `handler.go` — HTTP Handler

Implements `http.Handler`. Routes:

- **`GET /`** — Health check with configured model name
- **`POST /v1/embeddings`** — Main endpoint, OpenAI-format request/response
- **`OPTIONS /v1/embeddings`** — CORS preflight

Input parsing handles three formats: typed single, typed batch, and raw JSON fallback.

### `cmd/embedrock/main.go` — CLI

Parses flags, creates embedder, starts server. Version info injected at build time via ldflags.

## Design Decisions

### Why an interface?
Testability. All handler tests use `MockEmbedder` — no AWS credentials needed. The handler doesn't know about Bedrock.

### Why auto-detect model family?
One binary, one flag (`--model`). No separate "mode" or "provider" config. The model ID already encodes which family it belongs to.

### Why localhost-only by default?
The proxy uses the EC2 instance profile — exposing it would let anyone embed on your AWS bill. Use `--host 0.0.0.0` explicitly if needed.

### Why Go?
- Single static binary (~9MB), zero runtime dependencies
- Cross-compiles trivially (linux/arm64, linux/amd64, darwin/*)
- ~2MB RSS vs ~80MB for Node.js equivalent
- Perfect for `curl | install` deployment

## Testing

16 tests across two files:

- `bedrock_test.go` — model detection (`isCohere`, `isCohereV4`)
- `handler_test.go` — health, single/batch embeddings, Cohere v4, model passthrough, default fallback, error propagation, CORS, invalid methods/paths/bodies

```bash
go test ./... -v
```
