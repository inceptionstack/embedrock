# embedrock 🪨 — ARCHIVED

> **This project has been archived.** Its functionality has been superseded by **[bedrockify](https://github.com/inceptionstack/bedrockify)**, which provides both OpenAI-compatible **chat completions** AND **embeddings** backed by Amazon Bedrock — in a single binary.

## Migration

Replace `embedrock` with `bedrockify`:

```bash
# Install bedrockify
curl -fsSL https://github.com/inceptionstack/bedrockify/releases/latest/download/install.sh | bash

# Run as daemon (chat + embeddings on one port)
sudo bedrockify install-daemon \
  --region us-east-1 \
  --model us.anthropic.claude-opus-4-6-v1 \
  --embed-model amazon.titan-embed-text-v2:0

sudo systemctl daemon-reload && sudo systemctl enable --now bedrockify
```

The embeddings endpoint is the same: `POST /v1/embeddings` — no client changes needed.

**[→ bedrockify](https://github.com/inceptionstack/bedrockify)**
