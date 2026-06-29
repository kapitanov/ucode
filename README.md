# ucode

A terminal-based AI coding assistant powered by OpenRouter API.

## Features

- Interactive CLI for AI-assisted coding
- Tool execution: shell commands, file operations, code search
- Automatic conversation compaction to manage context window
- Markdown rendering with syntax highlighting
- Spinner animation during API calls

## Requirements

- Go 1.26+
- `rg` (ripgrep) for code search functionality
- OpenRouter API key

## Installation

```bash
git clone https://github.com/kapitanov/ucode.git
cd ucode
make build
```

Binary will be placed in `.out/ucode`.

## Configuration

Set your OpenRouter API key via environment variable or `.env` file:

```bash
export OPENROUTER_API_KEY=your_api_key_here
```

Or create a `.env` file in the project root:

```
OPENROUTER_API_KEY=your_api_key_here
OPENROUTER_MODEL=anthropic/claude-opus-4.5
```

## Usage

```bash
# Run with default settings
make run

# Or run the binary directly
./.out/ucode

# With custom options
./.out/ucode --api-key YOUR_KEY --model-name anthropic/claude-sonnet-4
```

### Command-line flags

| Flag | Description | Default |
|------|-------------|---------|
| `--api-key` | OpenRouter API key | `$OPENROUTER_API_KEY` |
| `--model-name` | Model to use | `anthropic/claude-opus-4.5` |
| `--max-messages` | Max messages before compaction | `50` |
| `--compacted-messages` | Messages kept after compaction | `20` |

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
