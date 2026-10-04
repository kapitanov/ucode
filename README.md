# ucode

A terminal-based AI coding assistant powered by OpenRouter.
Interactive CLI for AI-assisted coding with support for tool execution, context management, and agent orchestration.

## Features

- 🤖 **OpenRouter Integration** - Powered exclusively by OpenRouter API for LLM capabilities
- 🛠️ **Comprehensive Toolset** - 15 built-in tools for file operations, shell execution, code search,
  planning, memory management, and subagent orchestration
- 💾 **Smart Context Management** - Persistent planning and memory system for managing complex tasks
- 🎨 **Rich Terminal UI** - Interactive command interface with colored output and spinner animations
- 🔄 **Multi-Agent Architecture** - Support for specialized agent roles with YAML-based configuration
  and custom tool restrictions
- 📋 **Planning & Memory** - Persistent task planning and context-aware memory system for long-running sessions
- 🚀 **Intelligent Tool Execution** - Comprehensive toolset for working with files, shell commands,
  and subagent coordination

## Requirements

- **Go 1.26** - for building the project
- **ripgrep (rg)** - required for the `grep_files` tool to search code and text
- **patch** (GNU/BSD patch) - required for the `edit_file` tool to apply diffs
- **OpenRouter API key** - active OpenRouter account with API access

## Installation

```bash
git clone https://github.com/kapitanov/ucode.git
cd ucode
make build
```

The compiled binary will be placed in `.out/ucode`.

## Configuration

### OpenRouter Setup

Set your OpenRouter credentials via environment variables:

```bash
export OPENROUTER_URL="https://openrouter.ai/api/v1"
export OPENROUTER_API_KEY="your_api_key_here"
```


## Usage

### Quick Start

```bash
# Using environment variables
./.out/ucode

# Explicit URL and API key
./.out/ucode --url "https://openrouter.ai/api/v1" --key "your_api_key_here"
```

### CLI Flags

| Flag      | Description             | Source                            |
| --------- | ----------------------- | --------------------------------- |
| `--url`   | OpenRouter API base URL | `$OPENROUTER_URL` or required     |
| `--key`   | OpenRouter API key      | `$OPENROUTER_API_KEY` or required |
| `--model` | OpenRouter API model    | `$OPENROUTER_MODEL` or optional   |
| `--modern-ui` | Включить современный интерфейс с горячими клавишами | по умолчанию выключен |

**Note:** Both `--url` and `--key` can be provided via CLI flags or environment variables.
If not provided via flags, the tool will attempt to use environment variables.

### Environment Variables

| Variable             | Description                           | Example                        |
| -------------------- | ------------------------------------- | ------------------------------ |
| `OPENROUTER_URL`     | OpenRouter API base URL               | `https://openrouter.ai/api/v1` |
| `OPENROUTER_API_KEY` | OpenRouter API key for authentication | `sk-or-v1-xxxxxx...`           |
| `OPENROUTER_MODEL`   | OpenRouter API model                  | `gpt-4`                        |

## Interactive Commands

When running ucode, you can use the following commands:

- **All commands must be prefixed with a slash** (e.g., `/exit`).
  Input without a leading slash is treated as a prompt and sent to the AI agent.
- **Type your prompt** and press Enter to send a message to the AI agent
- **`/exit` or `/quit`** - Quit the application
- **`/quit`** - Quit the application (alias for /exit)
- **`/plan`** - View the current task plan
- **`/memory`** - View stored memory entries
- **`/usage`** - View token usage and cost

### Hotkeys (Modern UI)

When running with the `--modern-ui` flag, the following hotkeys are available:

- **Enter** — Отправить текущий ввод
- **Tab** — Переключение панелей
- **Ctrl+C** — Выйти из приложения
- **Alt+Enter** — Новая строка в вводе
- **Стрелки (↑/↓/←/→)** — Навигация
- **Home / End** — Перейти в начало / конец строки
- **Ctrl+U** — Очистить ввод
- **Ctrl+D** — Выйти из приложения на пустом поле
- **PgUp / PgDn** — Прокрутка чата

## Available Tools

The agent has access to the following 15 tools for code assistance and task execution:

### File Operations

- **`read_file`** - Read the contents of files in the workspace
- **`write_file`** - Create or overwrite files with new content
- **`edit_file`** - Edit files using diff-based patching for precise modifications
- **`list_files`** - List directory contents with optional glob pattern filtering
- **`grep_files`** - Search for text patterns across files using ripgrep (rg)
- **`rm_file`** - Remove a file from the workspace

### Shell Operations

- **`shell`** - Execute shell commands in the current working directory with support for complex operations

### Planning & Task Management

- **`write_plan`** - Create or update a plan with multiple task items
- **`read_plan`** - Read the current task plan
- **`check_plan`** - Mark plan items as completed to track progress
- **`clear_plan`** - Clear the entire task plan to start fresh

### Context & Memory

- **`read_memory`** - Access stored context information and previous findings
- **`write_memory`** - Store important information for later use in the session

### Agent Coordination

- **`ask_user`** - Prompt the user with a question and predefined options for interactive decision-making
- **`run_subagent`** - Spawn specialized subagents (e.g., for code review, for analysis) to handle specific tasks

## Architecture Overview

**ucode** is built with a modular, layered architecture:

```
┌─────────────────────────────────────────────────────────────┐
│                   Terminal UI (TUI)                         │
│    (Interactive prompts, colored output, animations)        │
└────────────────────────────┬────────────────────────────────┘
                             │
┌────────────────────────────▼────────────────────────────────┐
│                    Agent Runner                             │
│  (Orchestrates agent execution and tool coordination)       │
└────────────────────────────┬────────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
    ┌────────────┐    ┌──────────────┐    ┌──────────────┐
    │   Agents   │    │  Tools       │    │ LLM Provider │
    │            │    │              │    │              │
    │ - Multiple │    │ - File Ops   │    │ - OpenRouter │
    │   roles    │    │ - Shell      │    │              │
    │ - YAML     │    │ - Search     │    │              │
    │   config   │    │ - Subagents  │    │              │
    │ - Custom   │    │ - Planning   │    │              │
    │   tools    │    │ - Memory     │    │              │
    │            │    │ - Ask User   │    │              │
    └────────────┘    └──────────────┘    └──────────────┘
```

**Key Components:**

- **Agents** - Specialized AI agents with different roles, configured via `agents.yaml`,
  each with custom tool access and personality
- **Runner** - Core orchestration engine that manages agent lifecycle, tool execution, and context flow
- **Tools** - Extensible 15-tool system providing file operations, shell execution, planning, memory,
  and agent coordination
- **LLM** - OpenRouter integration layer for LLM responses and tool calling
- **TUI** - Interactive terminal interface with colored output and spinner animations

## Agent Roles

The system includes multiple specialized agent roles defined in `internal/agents/agents.yaml`.

Each agent has:
- Custom tool access restrictions (some tools forbidden for specific roles)
- Role-specific personality and communication style
- Default model configuration (inherited from global defaults)
- Specialized system prompts in Russian

## How It Works

1. **Initialization** - User launches ucode with OpenRouter credentials (via flags or environment variables)
2. **Agent Selection** - Default agent (Михалыч) is loaded with its configuration and system prompt
3. **Interaction Loop** - User enters prompts in the terminal UI
4. **LLM Processing** - Agent sends context and prompt to OpenRouter API
5. **Tool Execution** - If the LLM suggests tool calls, the agent executes them (file operations, shell commands, etc.)
6. **Response Handling** - Agent processes LLM responses and tool results, updating context and memory
7. **Context Management** - Long conversations are automatically compacted to maintain efficiency

## Examples

### Code Assistance

```bash
./.out/ucode --url "https://openrouter.ai/api/v1" --key "your_key"
> Analyze the project structure and suggest improvements
```

### Local Development with Environment Variables

```bash
export OPENROUTER_URL="https://openrouter.ai/api/v1"
export OPENROUTER_API_KEY="your_key"
./.out/ucode
> Review the code quality in main.go and suggest refactoring
```

### Code Search and File Operations

```bash
./.out/ucode --url "https://openrouter.ai/api/v1" --key "your_key"
> Search for all TODO comments and create a task list
```

## License

MIT
