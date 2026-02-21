# ⌬ MyOpenCode

<p align="center"><img src="https://github.com/user-attachments/assets/9ae61ef6-70e5-4876-bc45-5bcb4e52c714" width="800"></p>

> **⚠️ Active Development Notice:** This is a modified fork of OpenCode with enhanced configuration and ongoing development. Features may change, break, or be incomplete. Use at your own risk.

A powerful terminal-based AI assistant for developers, providing intelligent coding assistance directly in your terminal.

## Overview

MyOpenCode is a Go-based CLI application that brings AI assistance to your terminal. It provides a TUI (Terminal User Interface) for interacting with various AI models to help with coding tasks, debugging, and more. This is a modified fork of the original OpenCode project with enhanced configuration capabilities and ongoing development.

<p>For a quick video overview, check out
<a href="https://www.youtube.com/watch?v=P8luPmEa1QI"><img width="25" src="https://upload.wikimedia.org/wikipedia/commons/0/09/YouTube_full-color_icon_%282017%29.svg"> OpenCode + Gemini 2.5 Pro: BYE Claude Code! I'm SWITCHING To the FASTEST AI Coder!</a></p>

<a href="https://www.youtube.com/watch?v=P8luPmEa1QI"><img width="550" src="https://i3.ytimg.com/vi/P8luPmEa1QI/maxresdefault.jpg"></a><p>

## Features

- **Interactive TUI**: Built with [Bubble Tea](https://github.com/charmbracelet/bubbletea) for a smooth terminal experience
- **Multiple AI Providers**: Support for OpenAI, Anthropic Claude, Google Gemini, AWS Bedrock, Groq, Azure OpenAI, and OpenRouter
- **Enhanced Configuration**: Improved configuration system with better flexibility and customization options
- **Session Management**: Save and manage multiple conversation sessions
- **Tool Integration**: AI can execute commands, search files, and modify code
- **Vim-like Editor**: Integrated editor with text input capabilities
- **Persistent Storage**: SQLite database for storing conversations and sessions
- **LSP Integration**: Language Server Protocol support for code intelligence
- **File Change Tracking**: Track and visualize file changes during sessions
- **External Editor Support**: Open your preferred editor for composing messages
- **Named Arguments for Custom Commands**: Create powerful custom commands with multiple named placeholders

## Installation

### Using Go

```bash
go install myopencode@latest
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/yourusername/myopencode.git
cd myopencode

# Build
go build -o myopencode

# Run
./myopencode
```

## Configuration

MyOpenCode features an enhanced configuration system with improved flexibility:

### Configuration Locations

MyOpenCode looks for configuration in the following locations (in order of priority):

1. `./.myopencode.json` (local directory - highest priority)
2. `$XDG_CONFIG_HOME/myopencode/.myopencode.json`
3. `$HOME/.myopencode.json`

### Enhanced Configuration Features

- **Hierarchical Configuration**: Multiple configuration files can be combined with local settings taking precedence
- **Improved Validation**: Better error messages and validation for configuration options
- **Dynamic Reloading**: Configuration can be reloaded without restarting the application
- **Template Support**: Configuration templates for different use cases

### Basic Configuration Structure

```json
{
  "data": {
    "directory": ".myopencode"
  },
  "providers": {
    "openai": {
      "apiKey": "your-api-key",
      "disabled": false
    },
    "anthropic": {
      "apiKey": "your-api-key",
      "disabled": false
    }
  },
  "agents": {
    "coder": {
      "model": "claude-3.7-sonnet",
      "maxTokens": 5000
    }
  },
  "debug": false,
  "autoCompact": true
}
```

### Environment Variables

All configuration options can also be set via environment variables using the `MYOPENCODE_` prefix:

```bash
export MYOPENCODE_PROVIDERS_OPENAI_APIKEY="your-api-key"
export MYOPENCODE_AGENTS_CODER_MODEL="claude-3.7-sonnet"
export MYOPENCODE_DEBUG="true"
```

## Usage

```bash
# Start MyOpenCode
myopencode

# Start with debug logging
myopencode -d

# Start with a specific working directory
myopencode -c /path/to/project

# Run a single prompt in non-interactive mode
myopencode -p "Explain the use of context in Go"
```

## Non-interactive Prompt Mode

You can run MyOpenCode in non-interactive mode by passing a prompt directly as a command-line argument:

```bash
# Run a single prompt and print the AI's response to the terminal
myopencode -p "Explain the use of context in Go"

# Get response in JSON format
myopencode -p "Explain the use of context in Go" -f json

# Run without showing the spinner (useful for scripts)
myopencode -p "Explain the use of context in Go" -q
```

## Command-line Flags

| Flag              | Short | Description                                         |
| ----------------- | ----- | --------------------------------------------------- |
| `--help`          | `-h`  | Display help information                            |
| `--debug`         | `-d`  | Enable debug mode                                   |
| `--cwd`           | `-c`  | Set current working directory                       |
| `--prompt`        | `-p`  | Run a single prompt in non-interactive mode         |
| `--output-format` | `-f`  | Output format for non-interactive mode (text, json) |
| `--quiet`         | `-q`  | Hide spinner in non-interactive mode                |

## Development

### Prerequisites

- Go 1.24.0 or higher

### Building and Testing

```bash
# Build the project
go build -o myopencode

# Run tests
go test ./...

# Run with specific configuration
MYOPENCODE_DEBUG=true ./myopencode
```

### Project Structure

- `cmd/`: Command-line interface using Cobra
- `internal/app/`: Core application services
- `internal/config/`: Enhanced configuration management
- `internal/db/`: Database operations and migrations
- `internal/llm/`: LLM providers and tools integration
- `internal/tui/`: Terminal UI components and layouts
- `internal/logging/`: Logging infrastructure

## Differences from Original OpenCode

1. **Enhanced Configuration**: More flexible configuration system with hierarchical support
2. **Environment Variable Prefix**: Uses `MYOPENCODE_` prefix instead of generic environment variables
3. **Configuration File Name**: Uses `.myopencode.json` instead of `.opencode.json`
4. **Active Development**: This fork is actively maintained and developed
5. **Improved Error Handling**: Better validation and error messages for configuration

## Roadmap

- [ ] Add plugin system for extending functionality
- [ ] Improve configuration validation and schema
- [ ] Add more AI provider integrations
- [ ] Enhance TUI with more customization options
- [ ] Add configuration migration tools

## Contributing

Contributions are welcome! Here's how you can contribute:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

Please make sure to update tests as appropriate and follow the existing code style.

## License

MyOpenCode is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Acknowledgments

MyOpenCode is based on the original [OpenCode](https://github.com/opencode-ai/opencode) project. Special thanks to:

- The original OpenCode developers and contributors
- [@isaacphi](https://github.com/isaacphi) - For the [mcp-language-server](https://github.com/isaacphi/mcp-language-server) project
- [@adamdottv](https://github.com/adamdottv) - For the design direction and UI/UX architecture
- The Charm team for the excellent Bubble Tea framework