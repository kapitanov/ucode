# ucode

A terminal-based AI coding assistant powered by OpenRouter, OpenAI-compatible APIs, or Ollama.

## Features

- Interactive CLI for AI-assisted coding
- Support for multiple LLM providers: OpenRouter, OpenAI-compatible APIs, Ollama
- Tool execution: shell commands, file operations, code search
- Automatic conversation compaction to manage context window
- Markdown rendering with syntax highlighting
- Spinner animation during API calls

## Requirements

- Go 1.26+
- `rg` (ripgrep) for code search functionality
- An LLM provider: OpenRouter API key, an OpenAI-compatible endpoint, or Ollama running locally

## Installation

```bash
git clone https://github.com/kapitanov/ucode.git
cd ucode
make build
```

Binary will be placed in `.out/ucode`.

## Configuration

### OpenRouter

Set your OpenRouter API key via environment variable or `.env` file:

```bash
export OPENROUTER_API_KEY=your_api_key_here
```

Or create a `.env` file in the project root:

```
OPENROUTER_API_KEY=your_api_key_here
OPENROUTER_MODEL=anthropic/claude-opus-4.5
```

### OpenAI-compatible API (OpenAI, LM Studio, vLLM, etc.)

```
OPENAI_API_KEY=your_api_key_here
OPENAI_API_URL=http://127.0.0.1:1234/v1
OPENROUTER_MODEL=your-model-name
```

### Ollama

Make sure Ollama is running locally (default: `http://localhost:11434`).

```
OLLAMA_URL=http://localhost:11434
OPENROUTER_MODEL=llama3
```

## Usage

```bash
# Run the binary directly — --provider is required
./.out/ucode --provider openrouter --model anthropic/claude-sonnet-4

# With OpenRouter (key from env)
./.out/ucode --provider openrouter --model anthropic/claude-sonnet-4

# With OpenRouter (key inline)
./.out/ucode --provider openrouter --key YOUR_KEY --model anthropic/claude-sonnet-4

# With OpenAI API
./.out/ucode --provider openai --key YOUR_KEY --model gpt-4o

# With a local OpenAI-compatible server (e.g. LM Studio)
./.out/ucode --provider openai --url http://127.0.0.1:1234/v1 --key NONE --model qwen/qwen3.5-9b

# With Ollama
./.out/ucode --provider ollama --model llama3

# With Ollama on a custom host
./.out/ucode --provider ollama --url http://ollama-host:11434 --model llama3
```

### Command-line flags

| Flag | Description | Default |
|------|-------------|---------|
| `--provider` | LLM provider (`openrouter`, `openai`, or `ollama`) | *(required)* |
| `--url` | Provider base URL | see per-provider env vars below |
| `--key` | API key | see per-provider env vars below |
| `--model` | Model to use | `$OPENROUTER_MODEL` or `anthropic/claude-haiku-4.5` |
| `--max-messages` | Max messages before compaction | `100` |
| `--compacted-messages` | Messages kept after compaction | `25` |

### Environment variables

| Variable | Used by | Description |
|----------|---------|-------------|
| `OPENROUTER_API_KEY` | `openrouter` | OpenRouter API key |
| `OPENROUTER_API_URL` | `openrouter` | OpenRouter base URL (optional override) |
| `OPENAI_API_KEY` | `openai` | OpenAI API key |
| `OPENAI_API_URL` | `openai` | OpenAI base URL (for compatible APIs) |
| `OLLAMA_URL` | `ollama` | Ollama base URL (default: `http://localhost:11434`) |
| `OPENROUTER_MODEL` | all | Default model name |

### Interactive commands

- Type your prompt and press Enter to send
- `exit` or `/exit` - quit the application

## Available tools

The agent has access to:

- **shell** - Execute shell commands
- **ls** - List directory contents
- **read** - Read file contents
- **edit** - Edit/create files
- **codesearch** - Search code with ripgrep

## License

MIT
