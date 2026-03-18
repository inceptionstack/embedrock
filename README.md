# embedrock 🪨

OpenAI-compatible embedding proxy for Amazon Bedrock. Drop-in replacement for any tool expecting `/v1/embeddings`.

## Why?

Many AI tools (OpenClaw, LangChain, LlamaIndex, etc.) expect OpenAI's embedding API format. embedrock translates those calls to Amazon Bedrock, so you can use Titan, Cohere, and other Bedrock embedding models without changing your tools.

Zero API keys needed — uses your AWS credentials (instance profile, env vars, or shared config).

## Supported Models

| Model | ID | Dims | Notes |
|-------|-----|------|-------|
| Titan Embed Text V2 | `amazon.titan-embed-text-v2:0` | 1024 | Default |
| Titan Embed G1 Text | `amazon.titan-embed-g1-text-02` | 1536 | |
| Cohere Embed English v3 | `cohere.embed-english-v3` | 1024 | |
| Cohere Embed Multilingual v3 | `cohere.embed-multilingual-v3` | 1024 | |
| **Cohere Embed v4** | `cohere.embed-v4:0` | 1536 | Latest, best quality |

Model family is auto-detected by ID prefix. Titan and Cohere use different Bedrock request/response formats — embedrock handles this transparently.

> **Single-model design:** Each embedrock process serves one model. If a request specifies a different model, it returns HTTP 400 with a clear error. Run multiple instances for multiple models.

## Install

**From releases:**

```bash
# Linux arm64 (EC2 Graviton)
curl -fsSL https://github.com/inceptionstack/embedrock/releases/latest/download/embedrock-linux-arm64 -o /usr/local/bin/embedrock
chmod +x /usr/local/bin/embedrock

# Linux amd64
curl -fsSL https://github.com/inceptionstack/embedrock/releases/latest/download/embedrock-linux-amd64 -o /usr/local/bin/embedrock
chmod +x /usr/local/bin/embedrock

# macOS arm64 (Apple Silicon)
curl -fsSL https://github.com/inceptionstack/embedrock/releases/latest/download/embedrock-darwin-arm64 -o /usr/local/bin/embedrock
chmod +x /usr/local/bin/embedrock

# macOS amd64
curl -fsSL https://github.com/inceptionstack/embedrock/releases/latest/download/embedrock-darwin-amd64 -o /usr/local/bin/embedrock
chmod +x /usr/local/bin/embedrock
```

**From source:**

```bash
go install github.com/inceptionstack/embedrock/cmd/embedrock@latest
```

## Usage

```bash
# Default: localhost:8089, us-east-1, Titan Embed v2
embedrock

# Cohere Embed v4 (recommended for best quality)
embedrock --model cohere.embed-v4:0

# Custom config
embedrock --port 9090 --region eu-west-1 --model cohere.embed-english-v3

# Show version
embedrock --version

# All flags
embedrock --help
```

### CLI Reference

| Flag | Default | Description |
|------|---------|-------------|
| `--port` | `8089` | Port to listen on |
| `--host` | `127.0.0.1` | Host to bind to |
| `--region` | `us-east-1` | AWS region for Bedrock |
| `--model` | `amazon.titan-embed-text-v2:0` | Bedrock embedding model ID |
| `--version` | | Show version and exit |

### Subcommands

| Command | Description |
|---------|-------------|
| `embedrock update` | Self-update to the latest release from GitHub |
| `sudo embedrock install-daemon` | Install as a systemd service |

## Self-Update

embedrock can update itself in place:

```bash
embedrock update
```

This will:
1. Check GitHub for the latest release
2. Download the correct binary for your OS/architecture
3. Verify the SHA-256 checksum
4. Replace the current binary atomically
5. Restart the systemd service if running as one (requires root)

If running as a non-root user with a systemd service:
```
Updated embedrock from v0.3.0 to v0.4.0
Restart embedrock.service manually (requires sudo)
```

## Install as Daemon

Install embedrock as a systemd service with one command:

```bash
# Install with defaults (port 8089, us-east-1, Titan v2)
sudo embedrock install-daemon

# Install with custom settings
sudo embedrock --port 9090 --region eu-west-1 --model cohere.embed-v4:0 install-daemon
```

This will:
1. Copy the binary to `/usr/local/bin/embedrock`
2. Write a systemd unit file to `/etc/systemd/system/embedrock.service`
3. Run `systemctl daemon-reload`, `enable`, and `start`

The CLI flags you pass (`--port`, `--region`, `--model`) are baked into the service's `ExecStart` command.

To check service status:
```bash
systemctl status embedrock
```

## API

**Health check:**

```bash
curl http://127.0.0.1:8089/
# {"status":"ok","model":"amazon.titan-embed-text-v2:0"}
```

**Single embedding:**

```bash
curl -X POST http://127.0.0.1:8089/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{"input": "Hello world", "model": "amazon.titan-embed-text-v2:0"}'
```

**Batch embeddings:**

```bash
curl -X POST http://127.0.0.1:8089/v1/embeddings \
  -H "Content-Type: application/json" \
  -d '{"input": ["First text", "Second text"], "model": "amazon.titan-embed-text-v2:0"}'
```

**Response format:**

```json
{
  "object": "list",
  "data": [
    {
      "object": "embedding",
      "index": 0,
      "embedding": [0.123, -0.456, ...]
    }
  ],
  "model": "amazon.titan-embed-text-v2:0",
  "usage": {
    "prompt_tokens": 3,
    "total_tokens": 3
  }
}
```

**Error responses:**

```json
{
  "error": {
    "message": "model 'cohere.embed-v4:0' is not available; this server is configured with 'amazon.titan-embed-text-v2:0'",
    "type": "invalid_request"
  }
}
```

Internal errors return a generic `"embedding failed"` message — no AWS internals are leaked to clients.

## OpenClaw Configuration

```json
"memorySearch": {
  "enabled": true,
  "provider": "openai",
  "remote": {
    "baseUrl": "http://127.0.0.1:8089/v1/",
    "apiKey": "not-needed"
  },
  "model": "cohere.embed-v4:0",
  "query": {
    "hybrid": { "enabled": true, "vectorWeight": 0.7, "textWeight": 0.3 }
  }
}
```

## Prerequisites

- AWS credentials with `bedrock:InvokeModel` permission
- Bedrock model access enabled for your chosen embedding model

## Development

```bash
# Install dependencies
go mod download

# Run tests
go test ./... -v -count=1

# Build
go build ./cmd/embedrock/

# Build with version info
go build -ldflags "-X main.version=v0.4.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" ./cmd/embedrock/

# Lint (requires golangci-lint)
golangci-lint run ./...
```

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for design details and [AGENTS.md](AGENTS.md) for AI coding agent guidelines.

## License

MIT
