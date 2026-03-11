# Architecture

## Overview

embedrock is an OpenAI-compatible embedding proxy for Amazon Bedrock. It translates standard `/v1/embeddings` API calls into Bedrock `InvokeModel` requests, allowing any tool that speaks the OpenAI embeddings API to use Bedrock models with zero code changes.

```
┌─────────────┐     POST /v1/embeddings      ┌─────────────┐     InvokeModel      ┌─────────────┐
│  OpenClaw   │ ──────────────────────────►  │  embedrock  │ ──────────────────►  │   Bedrock   │
│  (or any    │  { input: "text",            │  (Go binary) │  { inputText: ... } │   Runtime   │
│   client)   │    model: "titan..." }       │  port 8089   │                     │             │
│             │ ◄──────────────────────────  │             │ ◄──────────────────  │             │
│             │  { data: [{ embedding }] }   │             │  { embedding: [...] }│             │
└─────────────┘                               └─────────────┘                     └─────────────┘
```

## Components

### `types.go` — Data Types
All OpenAI-compatible request/response types, the `Embedder` interface, and error types. This is the contract layer.

- **`Embedder` interface** — single method `Embed(text string) ([]float64, error)`. Everything depends on this interface, making the Bedrock implementation swappable.
- **`MockEmbedder`** — test double implementing `Embedder` with configurable behavior.
- **Request types** — `EmbeddingRequest` (single string input) and `EmbeddingRequestBatch` (array input), matching OpenAI's API.
- **Response types** — `EmbeddingResponse`, `EmbeddingData`, `Usage` — exact OpenAI format.

### `handler.go` — HTTP Handler
The core HTTP server logic. Implements `http.Handler` so it can be used with any Go HTTP server or test harness.

- **`GET /`** — Health check, returns `{"status":"ok","model":"..."}`
- **`POST /v1/embeddings`** — Main endpoint, accepts OpenAI-format requests
- **`OPTIONS /v1/embeddings`** — CORS preflight
- **Input parsing** — handles both single string and array inputs, with fallback to raw JSON parsing for edge cases
- **Error handling** — returns OpenAI-compatible error format `{"error":{"message":"...","type":"..."}}`

### `bedrock.go` — Bedrock Embedder
The real `Embedder` implementation that calls Amazon Bedrock.

- Uses AWS SDK v2 (`aws-sdk-go-v2/service/bedrockruntime`)
- Authenticates via standard AWS credential chain (instance profile, env vars, shared credentials)
- Currently targets Titan Embed Text v2 request/response format
- Stateless — each `Embed()` call is independent

### `cmd/embedrock/main.go` — CLI Entry Point
Parses flags, creates the Bedrock embedder, starts the HTTP server.

- `--port` (default: 8089)
- `--host` (default: 127.0.0.1)
- `--region` (default: us-east-1)
- `--model` (default: amazon.titan-embed-text-v2:0)
- `--version` — prints version info and exits

## Design Decisions

### Why an interface for Embedder?
Testability. All HTTP handler tests use `MockEmbedder` — no AWS credentials or network calls needed. The handler doesn't know or care about Bedrock; it just calls `Embed()`.

### Why localhost-only by default?
Security. The proxy uses the EC2 instance profile for authentication — exposing it to the network would let anyone generate embeddings on your AWS bill. Use `--host 0.0.0.0` explicitly if you need network access.

### Why not Lambda + API Gateway?
For single-instance setups (like OpenClaw on EC2), localhost is simpler, faster (no cold starts), and free. A Lambda version could be built using the same `handler.go` — it already implements `http.Handler`.

### Why Go?
- Single static binary (~10MB), zero runtime dependencies
- Cross-compiles trivially (linux/arm64, linux/amd64, darwin/*)
- Tiny memory footprint (~5MB RSS vs ~80MB for Node.js)
- Perfect for `curl | install` deployment pattern

## Testing

Tests live in `handler_test.go` and cover:
- Health endpoint
- Single and batch embeddings
- Invalid methods and paths
- Empty/malformed input
- Embedder errors (propagation)
- Model passthrough (client-specified model in response)
- CORS headers

Run: `go test ./... -v`

## Future

- Support Cohere Embed models (different request/response format)
- Support Amazon Titan Embed v1
- Metrics endpoint (`/metrics` for Prometheus)
- Request logging with configurable verbosity
- Rate limiting
