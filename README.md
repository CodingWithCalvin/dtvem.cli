<div align="center">
  <img src="assets/logo.png" alt="dtvem logo" width="200">

  # dtvem - Development Tool Virtual Environment Manager

  A cross-platform virtual environment manager for multiple developer tools, written in Go, with first-class support for Windows, MacOS, and Linux - right out of the box.

  [![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-blue?style=for-the-badge)]()
  [![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?style=for-the-badge&logo=go)]()
  [![License](https://img.shields.io/badge/license-MIT-green?style=for-the-badge)]()
  [![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow?style=for-the-badge)](https://conventionalcommits.org)

  [![Build & Test](https://img.shields.io/github/actions/workflow/status/dtvem/dtvem/build.yml?style=for-the-badge&label=Build%20%26%20Test)](https://github.com/dtvem/dtvem/actions)
  [![Release](https://img.shields.io/github/actions/workflow/status/dtvem/dtvem/release.yml?style=for-the-badge&label=Release)](https://github.com/dtvem/dtvem/actions)

  **[Documentation](https://github.com/dtvem/dtvem/wiki)** · **[Installation](https://github.com/dtvem/dtvem/wiki/Installation)** · **[Quick Start](https://github.com/dtvem/dtvem/wiki/Quick-Start)** · **[Commands](https://github.com/dtvem/dtvem/wiki/Commands)**

</div>

## 🤔 Why dtvem?

Managing multiple versions of Python, Node.js, Ruby, and other runtimes across different projects is painful. Existing tools like `nvm`, `pyenv`, and `rbenv` work great on Unix systems but have limited or no Windows support. **dtvem** solves this by providing a single, unified tool that works seamlessly across all platforms.

### Key Features

✅ **Cross-Platform**: Windows, Linux, and macOS with identical behavior

✅ **Multiple Runtimes**: Python, Node.js (Ruby, Go, and more coming)

✅ **Shim-Based**: Automatic version switching without shell integration

✅ **Migration Tool**: Import existing installations from nvm, pyenv, etc.

✅ **Per-Directory Versions**: `.dtvem/runtimes.json` for project-specific versions

✅ **No Shell Hooks**: Works in cmd.exe, PowerShell, bash, zsh, fish, etc.

See [Competitive Analysis](https://github.com/dtvem/dtvem/wiki/Competitive-Analysis) for how dtvem compares to nvm, pyenv, asdf, and mise.

## 📦 Installation

**macOS / Linux:**
```bash
curl -fsSL https://github.com/dtvem/dtvem/releases/latest/download/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://github.com/dtvem/dtvem/releases/latest/download/install.ps1 | iex
```

See [Installation Guide](https://github.com/dtvem/dtvem/wiki/Installation) for manual installation, building from source, and PATH configuration.

## 🚀 Quick Start

```bash
# Install a runtime version
dtvem install python 3.11.0
dtvem install node 20.0.0

# Set global default
dtvem global python 3.11.0

# Set project-local version
dtvem local node 18.16.0

# See what's active
dtvem current

# Migrate from nvm/pyenv
dtvem migrate node
dtvem migrate python
```

See [Quick Start Guide](https://github.com/dtvem/dtvem/wiki/Quick-Start) for more examples.

## 📚 Documentation

| Topic | Description |
|-------|-------------|
| [Installation](https://github.com/dtvem/dtvem/wiki/Installation) | Install on Windows, macOS, or Linux |
| [Quick Start](https://github.com/dtvem/dtvem/wiki/Quick-Start) | Get up and running in 5 minutes |
| [Commands](https://github.com/dtvem/dtvem/wiki/Commands) | Complete command reference |
| [Configuration](https://github.com/dtvem/dtvem/wiki/Configuration) | Config files and environment variables |
| [Architecture](https://github.com/dtvem/dtvem/wiki/Architecture) | How dtvem works (shims, version resolution) |
| [Migration](https://github.com/dtvem/dtvem/wiki/Migration) | Import from nvm, pyenv, fnm, etc. |
| [FAQ](https://github.com/dtvem/dtvem/wiki/FAQ) | Frequently asked questions |
| [Competitive Analysis](https://github.com/dtvem/dtvem/wiki/Competitive-Analysis) | vs nvm, pyenv, asdf, mise |
| [Roadmap](https://github.com/dtvem/dtvem/wiki/Roadmap) | Planned features and runtimes |

## 🤝 Contributing

Contributions are welcome! See the [Development Guide](https://github.com/dtvem/dtvem/wiki/Development) for:

- Setting up your development environment
- npm scripts for building and testing
- Commit conventions and PR guidelines
- CI/CD workflows and release process
- Adding new runtime providers

### Quick Setup

```bash
# First, install dtvem (see Installation section above)
# Then clone and set up the development environment:
git clone https://github.com/dtvem/dtvem.git
cd dtvem
dtvem install      # Install Node.js for git hooks
npm install        # Set up dev dependencies
npm run check      # Run format, lint, and tests
```

Looking for something to work on? Check out [good first issues](https://github.com/dtvem/dtvem/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22).

## 👥 Contributors

<!-- readme: contributors -start -->
<!-- readme: contributors -end -->

## 📄 License

MIT License - See [LICENSE](LICENSE) file for details
