# embedrock 🪨

OpenAI-compatible embedding proxy for Amazon Bedrock. Drop-in replacement for any tool expecting `/v1/embeddings`.

## Why?

Many AI tools (OpenClaw, LangChain, LlamaIndex, etc.) expect OpenAI's embedding API format. embedrock translates those calls to Amazon Bedrock, so you can use Titan Embed, Cohere Embed, and other Bedrock models without changing your tools.

Zero API keys needed — uses your AWS credentials (instance profile, env vars, or shared config).

## Install

**From releases:**
```bash
# Linux arm64 (EC2 Graviton)
curl -fsSL https://github.com/inceptionstack/embedrock/releases/latest/download/embedrock-linux-arm64 -o /usr/local/bin/embedrock
chmod +x /usr/local/bin/embedrock

# Linux amd64
curl -fsSL https://github.com/inceptionstack/embedrock/releases/latest/download/embedrock-linux-amd64 -o /usr/local/bin/embedrock
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

# Custom config
embedrock --port 9090 --region eu-west-1 --model cohere.embed-english-v3

# Flags
embedrock --help
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

## Run as a Service

```bash
sudo tee /etc/systemd/system/embedrock.service > /dev/null << 'EOF'
[Unit]
Description=embedrock - Bedrock embedding proxy
After=network.target

[Service]
Type=simple
User=ec2-user
ExecStart=/usr/local/bin/embedrock --port 8089 --region us-east-1
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable embedrock
sudo systemctl start embedrock
```

## OpenClaw Configuration

```json
"memorySearch": {
  "enabled": true,
  "provider": "openai",
  "remote": {
    "baseUrl": "http://127.0.0.1:8089/v1/",
    "apiKey": "not-needed"
  },
  "model": "amazon.titan-embed-text-v2:0",
  "query": {
    "hybrid": { "enabled": true, "vectorWeight": 0.7, "textWeight": 0.3 }
  }
}
```

## Prerequisites

- AWS credentials with `bedrock:InvokeModel` permission
- Bedrock model access enabled for your chosen embedding model

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for design details.

## License

MIT
