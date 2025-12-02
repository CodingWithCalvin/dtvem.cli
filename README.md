<div align="center">
  <img src="assets/logo.png" alt="dtvem logo" width="200">

  # dtvem - Development Tool Virtual Environment Manager

  A cross-platform virtual environment manager for multiple developer tools, written in Go, with first-class support for Windows, MacOS, and Linux - right out of the box.

  [![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux%20%7C%20macOS-blue)]()
  [![Go Version](https://img.shields.io/badge/go-1.23+-00ADD8?logo=go)]()
  [![License](https://img.shields.io/badge/license-MIT-green)]()
  [![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg)](https://conventionalcommits.org)

  [![Build & Test](https://github.com/dtvem/dtvem/actions/workflows/build.yml/badge.svg)](https://github.com/dtvem/dtvem/actions)
  [![Lint](https://github.com/dtvem/dtvem/actions/workflows/lint.yml/badge.svg)](https://github.com/dtvem/dtvem/actions)
  [![Release](https://github.com/dtvem/dtvem/actions/workflows/release.yml/badge.svg)](https://github.com/dtvem/dtvem/actions)

</div>

## Why dtvem?

Managing multiple versions of Python, Node.js, Ruby, and other runtimes across different projects is painful. Existing tools like `nvm`, `pyenv`, and `rbenv` work great on Unix systems but have limited or no Windows support. **dtvem** solves this by providing a single, unified tool that works seamlessly across all platforms.

### Key Features

✅ **Cross-Platform**: Windows, Linux, and macOS with identical behavior

✅ **Multiple Runtimes**: Python, Node.js, Ruby, Go (extensible plugin system)

✅ **Shim-Based**: Automatic version switching without shell integration

✅ **Migration Tool**: Import existing installations from nvm, pyenv, etc.

✅ **Per-Directory Versions**: `.dtvem/runtimes.json` file for project-specific versions

✅ **Global Defaults**: Set system-wide default versions

✅ **No Shell Hooks**: Works in cmd.exe, PowerShell, bash, zsh, fish, etc.

## Installation

### Quick Install (Recommended)

**macOS / Linux:**
```bash
curl -fsSL https://github.com/dtvem/dtvem/releases/latest/download/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://github.com/dtvem/dtvem/releases/latest/download/install.ps1 | iex
```

The installer will:
- Download the latest release for your platform
- Install to `~/.dtvem/bin` (both Unix and Windows)
- Add `~/.dtvem/bin` to the beginning of your PATH (for dtvem command)
- Run `dtvem init` to add `~/.dtvem/shims` to PATH (for runtime commands)
- Display next steps

### Manual Installation

Download the appropriate binary from [GitHub Releases](https://github.com/dtvem/dtvem/releases/latest):

- **Windows (x64)**: `dtvem-X.X.X-windows-amd64.zip`
- **Windows (ARM64)**: `dtvem-X.X.X-windows-arm64.zip`
- **macOS Intel**: `dtvem-X.X.X-macos-amd64.tar.gz`
- **macOS Apple Silicon**: `dtvem-X.X.X-macos-arm64.tar.gz`
- **Linux**: `dtvem-X.X.X-linux-amd64.tar.gz`

Extract and move binaries to a directory in your PATH, then run:

```bash
dtvem init
```

### Building from Source

```bash
# Clone the repository
git clone https://github.com/dtvem/dtvem.git
cd dtvem

# Build both executables
go build -o dist/dtvem.exe ./src
go build -o dist/dtvem-shim.exe ./src/cmd/shim

# Run init to configure PATH
./dist/dtvem.exe init
```

**Manual PATH Configuration (Optional)**

If you prefer to configure PATH manually, you need to add both directories:

**Windows (PowerShell):**
```powershell
# Add dtvem binary directory (for dtvem command)
[Environment]::SetEnvironmentVariable("Path", "$HOME\.dtvem\bin;" + $env:Path, "User")
# Add shims directory (for runtime commands like python, node, etc.)
[Environment]::SetEnvironmentVariable("Path", "$HOME\.dtvem\shims;" + $env:Path, "User")
```

**Unix (bash/zsh):**
```bash
# Add both directories to the beginning of PATH
export PATH="$HOME/.dtvem/bin:$HOME/.dtvem/shims:$PATH"
```

Add this to your `~/.bashrc`, `~/.zshrc`, or equivalent shell configuration file.

**Note:** Both directories are added to the **beginning** of PATH to ensure dtvem-managed versions take precedence over system installations.

## Quick Start

```bash
# Initialize dtvem (first time setup)
dtvem init

# Migrate existing Node.js installation
dtvem migrate node

# Or install a specific version
dtvem install python 3.11.0

# Set global default version
dtvem global python 3.11.0

# Set local version for current project
dtvem local node 18.16.0

# Or create config from global versions
dtvem freeze

# Install all runtimes from .dtvem/runtimes.json
dtvem install          # Prompts for confirmation
dtvem install --yes    # Skip confirmation

# Check currently active versions
dtvem current

# List installed versions
dtvem list python

# List all available versions
dtvem list-all python

# List with filtering
dtvem list-all python --filter 3.11

# Uninstall a version
dtvem uninstall python 3.10.0

# Show where a command is located
dtvem which python

# Show installation directory
dtvem where python 3.11.0

# Regenerate shims (if needed)
dtvem reshim
```

## Architecture

### Directory Structure

```
~/.dtvem/                      # Root directory
├── bin/                       # dtvem binaries (add to PATH first)
│   ├── dtvem                  # Main CLI executable
│   └── dtvem-shim             # Shim template binary
├── shims/                     # Shim executables (add to PATH second)
│   ├── python.exe
│   ├── node.exe
│   ├── npm.exe
│   └── ...
├── versions/                  # Installed runtime versions
│   ├── python/
│   │   ├── 3.11.0/
│   │   └── 3.12.0/
│   └── node/
│       ├── 18.16.0/
│       └── 22.0.0/
└── config/
    └── runtimes.json          # Global version configuration
```

### How It Works

1. **Shim System**: All runtime executables are intercepted by small Go binaries (shims)
2. **Version Resolution**: Shims check for `.dtvem/runtimes.json` file (local) → global config
3. **PATH Fallback**: If no dtvem version configured, falls back to system installation
4. **Execution**: Shim executes the appropriate versioned binary (dtvem or system)
5. **No Shell Hooks**: Works in any shell without modification

```
User runs: python --version
    ↓
~/.dtvem/shims/python (shim intercepts)
    ↓
Reads .dtvem/runtimes.json → {"python": "3.11.0"}
    ↓
Executes: ~/.dtvem/versions/python/3.11.0/bin/python --version
```

### Auto-Install Missing Versions

When you run a command that requires a version that isn't installed yet, dtvem will offer to install it automatically:

```bash
$ python --version
⚠ Python 3.11.0 is not installed
→ Install it now? [Y/n]: y
→ Installing Python 3.11.0...
✓ Successfully installed Python 3.11.0
Python 3.11.0
```

This works seamlessly with bulk install - just clone a repo with a `.dtvem/runtimes.json` file and start using it. The first time you run a command, dtvem will offer to install the required version.

**Environment Variable Control:**
- `DTVEM_AUTO_INSTALL=false` - Disable auto-install in CI/automation
- `DTVEM_AUTO_INSTALL=true` - Auto-install without prompting
- Default - Interactive prompt

### System PATH Fallback

When no dtvem version is configured, shims transparently fall back to system installations. This makes dtvem non-disruptive and allows gradual migration:

```bash
# No dtvem version configured, but system Python exists
$ python --version
→ No dtvem version configured for Python
→ Using system installation: /usr/bin/python3
→ To manage with dtvem, run: dtvem install python <version>

Python 3.11.2
```

```bash
# No dtvem version and no system installation
$ python --version
⚠ No dtvem version configured for Python
⚠ No system installation found in PATH

→ To install with dtvem:
  1. See available versions: dtvem list-all python
  2. Install a version: dtvem install python <version>
  3. Set it globally: dtvem global python <version>
```

**Benefits:**
- Works out of the box with existing system installations
- Migrate runtimes one at a time at your own pace
- Doesn't break existing workflows
- Clear messaging shows what's happening

### Automatic Reshim for Global Packages

When you install global packages (like `typescript`, `eslint`, `black`, `pylint`), dtvem automatically detects it and offers to update shims:

```bash
$ npm install -g typescript
[... npm installation output ...]

→ Global packages were installed/removed
Run 'dtvem reshim' to update shims? [Y/n]: ← Press Enter
✓ Shims updated successfully

$ tsc --version  ← Works immediately!
Version 5.3.3
```

**How it works:**
- **Smart Detection**: Detects `npm install -g`, `pip install`, and similar commands
- **Automatic Prompting**: After successful installation, prompts to run reshim
- **One-Command Update**: Press Enter and shims are updated automatically
- **Seamless Experience**: Your new executables are immediately available

**Supported commands:**
- **Node.js**: `npm install/uninstall -g` (or `--global`)
- **Python**: `pip install/uninstall` (any package with executables)

**Manual reshim:**
If you decline the prompt or want to update shims manually:
```bash
dtvem reshim
```

This scans all installed versions and creates shims for every executable found, including globally installed packages.

## Commands

### Core Commands

| Command | Description | Status |
|---------|-------------|--------|
| `init` | Initialize dtvem (setup directories and PATH) | ✅ Complete |
| `install [runtime] [version]` | Install a specific runtime version, or all from `.dtvem/runtimes.json` | ✅ Complete |
| `uninstall <runtime> <version>` | Remove an installed version | ✅ Complete |
| `list <runtime>` | List installed versions | ✅ Complete |
| `list-all <runtime>` | List all available versions (with filtering) | ✅ Complete |

### Version Management

| Command | Description | Status |
|---------|-------------|--------|
| `global <runtime> <version>` | Set global default version | ✅ Complete |
| `local <runtime> <version>` | Set local version for current directory | ✅ Complete |
| `current [runtime]` | Show currently active version(s) | ✅ Complete |
| `freeze` | Create .dtvem/runtimes.json from global versions | ✅ Complete |

### Migration & Maintenance

| Command | Description | Status |
|---------|-------------|--------|
| `migrate <runtime>` | Import existing installations | ✅ Complete |
| `reshim` | Regenerate shim binaries (usually automatic) | ✅ Complete |
| `which <command>` | Show path to command and shim | ✅ Complete |
| `where <runtime> [version]` | Show install directory | ✅ Complete |

### Information

| Command | Description | Status |
|---------|-------------|--------|
| `version` | Show dtvem version | ✅ Complete |
| `help` | Show help | ✅ Complete |

## Configuration

### Local Version File (`.dtvem/runtimes.json`)

Create a `.dtvem` directory in your project with a `runtimes.json` file to specify runtime versions:

```json
{
  "python": "3.11.0",
  "node": "18.16.0"
}
```

**JSON Schema**: A [JSON schema file](schemas/runtimes.schema.json) is available for IDE autocomplete and validation. Most modern editors will automatically detect and use it.

To explicitly reference the schema in your config file:
```json
{
  "$schema": "https://raw.githubusercontent.com/dtvem/dtvem/main/schemas/runtimes.schema.json",
  "python": "3.11.0",
  "node": "18.16.0"
}
```

### Global Configuration (`~/.dtvem/config/runtimes.json`)

System-wide default versions (JSON format):

```json
{
  "python": "3.12.0",
  "node": "20.0.0"
}
```

### Environment Variables

- `DTVEM_ROOT`: Override default dtvem directory (default: `~/.dtvem`)
- `DTVEM_AUTO_INSTALL`: Control auto-install behavior when a configured version is missing
  - `false` - Disable auto-install prompts (useful for CI/automation)
  - `true` - Auto-install without prompting
  - Not set - Interactive prompt (default)

## Supported Runtimes

### Currently Implemented

| Runtime | Detection | Installation | Notes |
|---------|-----------|--------------|-------|
| **Node.js** | ✅ Complete | ✅ Complete | Windows/macOS/Linux, detects system/nvm/fnm |
| **Python** | ✅ Complete | ✅ Complete | Windows/macOS/Linux, detects system/pyenv |

### Planned

- Ruby
- Go
- Rust
- Java/JDK
- .NET SDK

## Migration

dtvem can detect and migrate existing runtime installations, including their global packages:

```bash
$ dtvem migrate node

🔍 Scanning for Node.js installations...

Found 3 installation(s):
  [1] v22.0.0  (system) /usr/local/bin/node ✓
  [2] v18.16.0 (nvm)    ~/.nvm/versions/node/v18.16.0/bin/node
  [3] v20.11.0 (fnm)    ~/.fnm/node-versions/v20.11.0/bin/node

Select versions to migrate:
  Enter numbers separated by commas, or 'all': 1,2

Migrating Node.js v22.0.0...
  → Detecting global packages... (typescript, eslint, prettier)
  → Installing Node.js v22.0.0...
  → Reinstalling 3 global package(s)...
  ✓ Installed successfully

Migration complete! 2/2 version(s) installed.

Cleanup Old Installations
→ You have successfully migrated to dtvem. Would you like to clean up the old installations?
→ This helps prevent PATH conflicts and version confusion.

Old installation: v22.0.0 (system)
  Location: /usr/local/bin/node
⚠ Manual removal required
→ To uninstall:
  If installed via Homebrew: brew uninstall node
  Manual removal: sudo rm -rf /usr/local/bin/node

Old installation: v18.16.0 (nvm)
  Location: ~/.nvm/versions/node/v18.16.0
Remove this installation? [y/N]: y
→ Removing Node.js v18.16.0 from nvm...
✓ Removed Node.js v18.16.0 from nvm

✓ Removed 1 old installation(s)
⚠ PATH Conflict Warning
→ You have 1 old installation(s) remaining
→ Consider removing them manually to avoid confusion
```

**Migration features:**
- **Global package preservation**: Automatically detects and reinstalls global packages
  - Node.js: npm packages (typescript, eslint, prettier, etc.)
  - Python: pip packages (black, pylint, pytest, etc.)
- **Old installation cleanup**: Prompts to remove old installations after migration
  - Automated removal for version managers (nvm, pyenv, fnm, rbenv)
  - OS-specific instructions for system installs
  - PATH conflict warnings for remaining installations

### Supported Migration Sources

**Node.js:**
- System installations (Windows, macOS, Linux)
- nvm (Node Version Manager)
- fnm (Fast Node Manager)

**Python:**
- System installations (Windows, macOS, Linux)
- pyenv
- pyenv-win (Windows)

## Plugin System

dtvem uses an embedded plugin system with a registry pattern. Each runtime provider owns its shim definitions (executables it provides), making the architecture fully decentralized:

```go
// Each runtime implements the Provider interface (19 methods)
type Provider interface {
    Name() string                                    // e.g., "python"
    DisplayName() string                             // e.g., "Python"
    Shims() []string                                 // e.g., ["python", "python3", "pip", "pip3"]
    ShouldReshimAfter(shimName, args) bool           // e.g., detect "npm install -g"
    Install(version string) error
    DetectInstalled() ([]DetectedVersion, error)
    // ... 13 more methods
}

// Runtimes auto-register on startup
func init() {
    runtime.Register(NewProvider())
}
```

**Adding a new runtime is as simple as:**
1. Create `src/runtimes/<name>/provider.go`
2. Implement the `Provider` interface (including `Shims()` and `ShouldReshimAfter()`)
3. Import in `src/main.go`

**That's it!** The shim mappings are automatically registered via the `Shims()` method, and automatic reshim detection works via `ShouldReshimAfter()`. No need to modify central mapping files.

## Development

### Prerequisites

- Go 1.23 or higher
- Git

### Building

```bash
# Build main executable
go build -o dist/dtvem.exe ./src

# Build shim executable
go build -o dist/dtvem-shim.exe ./src/cmd/shim

# Build for specific platforms
GOOS=linux GOARCH=amd64 go build -o dist/dtvem ./src
GOOS=darwin GOARCH=arm64 go build -o dist/dtvem ./src
GOOS=windows GOARCH=amd64 go build -o dist/dtvem.exe ./src
```

### Testing

```bash
# Run all tests
cd src && go test ./...

# Run tests for a specific package
cd src && go test ./internal/config -v

# Run tests with coverage
cd src && go test -cover ./...

# Generate HTML coverage report
cd src && go test -coverprofile=coverage.out ./...
cd src && go tool cover -html=coverage.out -o coverage.html
```

**Test Coverage (63+ tests):**
- `internal/config` - Path helpers, environment variables, version resolution, directory walking
- `internal/path` - PATH checking, shim directory resolution
- `internal/shim` - Runtime shim name mapping, file operations
- `internal/runtime` - Version types, registry, provider contract test harness
- `internal/testutil` - Reusable test utilities (string helpers)
- `internal/ui` - Output formatting functions
- `cmd` - Command validation and behavior (uninstall, migrate, etc.)
- `runtimes/node` - Node.js provider contract validation
- `runtimes/python` - Python provider contract validation

**Automated Coverage Tracking:**
- Every PR gets an automatic coverage report posted as a comment
- Coverage reports show total coverage and per-package breakdown
- HTML coverage reports available as workflow artifacts
- Coverage data tracked using `-covermode=atomic` for accuracy

### Continuous Integration

GitHub Actions automatically:
- Builds and tests on Windows, macOS, and Linux for every push/PR
- Generates coverage reports and posts summary to PRs
- Displays test results in workflow summaries
- Uploads HTML coverage reports as artifacts (retained for 30 days)
- Runs all tests before creating releases
- Creates releases with binaries for all platforms when you push a version tag

**PR Coverage Reports:**
When you open a pull request, the build workflow automatically:
- Runs full test suite with coverage tracking
- Generates detailed coverage reports (HTML + summary)
- Posts coverage summary as a PR comment (updated on each push)
- Uploads detailed HTML coverage report as a downloadable artifact

### Project Structure

```
dtvem/
├── src/
│   ├── cmd/                  # CLI commands
│   │   ├── shim/             # Shim executable
│   │   ├── init.go           # Init command
│   │   ├── install.go
│   │   ├── migrate.go
│   │   └── ...
│   ├── internal/
│   │   ├── config/           # Configuration & paths
│   │   ├── download/         # Download utilities
│   │   ├── path/             # PATH configuration (platform-specific)
│   │   ├── runtime/          # Plugin system core
│   │   ├── shim/             # Shim management
│   │   └── ui/               # Colored output & progress indicators
│   ├── runtimes/             # Runtime providers
│   │   ├── python/
│   │   └── node/
│   └── main.go
├── dist/                     # Build outputs
├── go.mod
└── README.md
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Areas for Contribution

- [ ] Add more runtime providers (Ruby, Go, Rust)
- [ ] Add shell completion scripts
- [ ] Improve documentation
- [ ] Write more tests

## Releasing

To create a new release:

1. **Ensure all tests pass locally:**
   ```bash
   cd src && go test ./...
   ```

2. **Commit and push your changes to `main` branch**

3. **Trigger the release workflow manually:**
   - Go to [Actions → Release](https://github.com/dtvem/dtvem/actions/workflows/release.yml)
   - Click "Run workflow"
   - Select the `main` branch
   - Enter the version number (e.g., `1.0.0`, `0.2.1`)
   - Click "Run workflow"

The release workflow will automatically:
- Validate the version format (must be `X.Y.Z`)
- Run all tests on Windows, macOS (amd64 + arm64), and Linux
- Build binaries for all platforms with the version embedded
- Create a git tag (`v1.0.0`) and push it to the repository
- Generate a changelog from commits since the last release
- Create a GitHub Release with binaries, install scripts, and changelog
- **Important:** Release is only created if all tests pass on all platforms

**Supported Release Platforms:**
- Windows (amd64, arm64)
- macOS (amd64, arm64)
- Linux (amd64)

## License

MIT License - See LICENSE file for details
