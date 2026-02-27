# ⌬ MyOpenCode

A powerful terminal-based AI assistant for developers, providing intelligent coding assistance directly in your terminal. This project focuses on high-performance AI interactions, robust response handling, and deep integration with developer workflows.

## Overview

MyOpenCode is a Go-based CLI application that brings AI assistance to your terminal. It provides a TUI (Terminal User Interface) for interacting with various AI models to help with coding tasks, debugging, and more. This fork focuses on flexibility, supporting dynamic AI providers and robust handling of long AI responses.

## Key Features

- **Dynamic AI Providers**: Easily add OpenAI-compatible providers (DeepSeek, Groq, Bailian, etc.) via JSON configuration.
- **Provider-Specific Model IDs**: Use the `provider::model` syntax (e.g., `deepseek::chat`) for unambiguous model selection.
- **Truncation Recovery**: Automatically detects truncated AI responses (e.g., hitting `max_tokens`) and triggers a "Continue" loop to merge results into a single, valid response.
- **DeepSeek R1 Support**: Full support for reasoning models, including thinking process display and context preservation.
- **Interactive TUI**: Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) featuring a split-view chat and a refined, real-time log explorer with text wrapping.
- **Session Management**: Save and manage multiple conversation sessions with SQLite persistence.
- **Tool Integration**: AI can execute commands, search files, and modify code based on your requirements.
- **Skills System**: Extensible "Skills" mechanism that allows the AI to discover and use domain-specific instructions and scripts from `~/.my-opencode/skills/`.
- **LSP Integration**: Language Server Protocol support for deep code intelligence.
- **Custom Max Tokens**: Configure per-model limits to stay within provider boundaries (e.g., DeepSeek's 8,192 token limit).

## Installation

### Using Go

```bash
go install github.com/markusleevip/my-opencode@latest
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/markusleevip/my-opencode.git
cd my-opencode

# Build
go build -o myopencode

# Run
./myopencode
```

## Configuration

MyOpenCode features an enhanced configuration system with improved flexibility:

### Configuration Locations

The application looks for configuration in the following locations (in order of priority):

1. `./settings.json` (local directory - highest priority)
2. `$HOME/.my-opencode/settings.json`
3. `$XDG_CONFIG_HOME/opencode/settings.json`

### Configuration Example (settings.json)

Here is a comprehensive example of `settings.json` based on a real-world setup, supporting multiple dynamic providers, custom TUI themes, and shell settings:

```json
{
  "data": {
    "directory": ".my-opencode"
  },
  "provider": {
    "deepseek": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Deepseek",
      "apiKey": "sk-...",
      "options": {
        "baseURL": "https://api.deepseek.com/v1"
      },
      "models": {
        "deepseek-chat": {
          "name": "Deepseek Chat"
        }
      }
    },
    "zhipu": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Zhipu",
      "options": {
        "baseURL": "https://api.z.ai/api/coding/paas/v4",
        "apiKey": "your-zhipu-api-key"
      },
      "models": {
        "glm-4.7": {
          "name": "GLM 4.7"
        }
      }
    },
    "bailian": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "Bailian",
      "apiKey": "sk-...",
      "options": {
        "baseURL": "https://dashscope.aliyuncs.com/compatible-mode/v1"
      },
      "models": {
        "qwen3.5-plus": {
          "name": "Qwen3.5 Plus",
          "apiModel": "qwen3.5-plus",
          "maxTokens": 8192,
          "contextWindow": 128000,
          "canReason": false,
          "supportsAttachments": true
        }
      }
    }
  },
  "tui": {
    "theme": "default"
  },
  "shell": {
    "path": "powershell.exe",
    "args": ["-NoProfile", "-NonInteractive", "-NoLogo", "-Command", "-"]
  },
  "autoCompact": true,
  "contextPaths": [
    ".github/copilot-instructions.md",
    ".cursorrules",
    "CLAUDE.md"
  ]
}
```

### Skills

MyOpenCode features a powerful "Skills" system that allows you to extend the assistant's capabilities with custom instructions and scripts. 

### How it Works

1. **Discovery**: On startup, the assistant automatically scans `~/.my-opencode/skills/` for subdirectories.
2. **Definition**: Each skill directory must contain a `SKILL.md` file with optional YAML frontmatter.
3. **Execution**: The assistant can read these instructions and, if provided, execute scripts located in the skill's `scripts/` directory.

### Skill Structure

```
~/.my-opencode/skills/
  └── my-skill/
      ├── SKILL.md       # Target instructions and metadata
      ├── scripts/       # Helper scripts the AI can run
      └── resources/     # Additional context files
```

### SKILL.md Example

```markdown
---
name: PDF Generator
description: Tools for converting markdown to professional PDF documents
---
Explain to the AI how to use this skill...
```

## Environment Variables

Configuration options can be set via environment variables using the `OPENCODE_` prefix:

```bash
export OPENCODE_PROVIDERS_OPENAI_APIKEY="your-api-key"
export OPENCODE_AGENTS_CODER_MODEL="deepseek::reasoner"
export OPENCODE_DEBUG="true"
```

## Usage

```bash
# Start MyOpenCode (Interactive TUI)
myopencode

# Switch models in TUI
# Type: /model deepseek

# Start with a specific working directory
myopencode -c /path/to/project

# Run a single prompt in non-interactive mode
myopencode -p "Fix the bug in main.go"
```

## Development

### Prerequisites

- Go 1.24.0 or higher

### Building and Testing

```bash
# Build the project
go build -o myopencode

# Run tests
go test ./...
```

## Differences from Original OpenCode

1. **Enhanced Provider Logic**: Support for dynamic, user-defined providers without code changes.
2. **Robust Content Merging**: Intelligent merging of parital JSON and text when responses are truncated.
3. **Optimized for Coding**: Specifically tuned for large context windows and long-running generation tasks.
4. **Improved Logging**: A high-performance log viewer designed for debugging complex AI agent interactions.

## License

MyOpenCode is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Acknowledgments

MyOpenCode is based on the original [OpenCode](https://github.com/opencode-ai/opencode) project. Special thanks to the original developers and the Charm team for the Bubble Tea framework.