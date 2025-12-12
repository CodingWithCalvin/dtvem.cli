# CLAUDE.md

Guidance for Claude Code when working with the dtvem codebase.

## Related Documentation

| Document | Purpose |
|----------|---------|
| [`README.md`](../README.md) | User-facing documentation, installation, usage examples |
| [`CONTRIBUTING.md`](../CONTRIBUTING.md) | Contribution guidelines, PR process, development setup |
| [`CODE_OF_CONDUCT.md`](../CODE_OF_CONDUCT.md) | Community standards and expected behavior |
| [`GO_STYLEGUIDE.md`](GO_STYLEGUIDE.md) | Complete Go coding standards (Google style) |

---

## Critical Rules

**These rules override all other instructions:**

1. **Follow the styleguide** - All code must comply with `GO_STYLEGUIDE.md`
2. **Write tests** - All new/refactored code requires comprehensive unit tests
3. **Cross-platform** - All features must work on Windows, macOS, and Linux
4. **Conventional commits** - Format: `type(scope): description`
5. **GitHub Issues for TODOs** - Use `gh` CLI to manage issues, no local TODO files
6. **Run validation before commits** - Run `npm run check` (format, lint, test) before committing and pushing

---

## Quick Reference

### Build Commands

```bash
# Build executables
go build -o dist/dtvem.exe ./src
go build -o dist/dtvem-shim.exe ./src/cmd/shim

# Run directly
go run ./src/main.go <command>

# Run tests
cd src && go test ./...
```

### Deploy After Building

```bash
cp dist/dtvem.exe ~/.dtvem/bin/dtvem.exe
cp dist/dtvem-shim.exe ~/.dtvem/bin/dtvem-shim.exe
~/.dtvem/bin/dtvem.exe reshim
```

### GitHub Issues

```bash
gh issue list                    # List open issues
gh issue view <number>           # View details
gh issue create --title "..." --label enhancement --body "..."
gh issue close <number>
```

---

## Project Overview

**dtvem** (Development Tool Virtual Environment Manager) is a cross-platform runtime version manager written in Go. Similar to `asdf` but with first-class Windows support.

| Attribute | Value |
|-----------|-------|
| Version | dev (pre-1.0) |
| Runtimes | Python, Node.js |
| Tests | 230+ passing |
| Style | Google Go Style Guide |

**Key Concept**: Shims are Go executables that intercept runtime commands (like `python`, `node`), resolve versions, and execute the appropriate binary.

### Available Commands

`init`, `install`, `uninstall`, `list`, `list-all`, `global`, `local`, `current`, `freeze`, `migrate`, `reshim`, `which`, `where`, `version`, `help`

---

## Architecture

### Two-Binary System

1. **Main CLI** (`src/main.go`) - The `dtvem` command
2. **Shim Executable** (`src/cmd/shim/main.go`) - Copied/renamed for each runtime command

### Shim Flow

```
User runs: python --version
    ↓
~/.dtvem/shims/python.exe (shim)
    ↓
Maps to runtime provider → Resolves version
    ↓
├─ Version configured & installed? → Execute ~/.dtvem/versions/python/3.11.0/bin/python
├─ Version configured but not installed? → Show error with install command
└─ No version configured? → Fall back to system PATH or show install instructions
```

### Directory Structure

```
~/.dtvem/
├── bin/            # dtvem binaries (added to PATH)
│   ├── dtvem
│   └── dtvem-shim
├── shims/          # Runtime shims (python.exe, node.exe, etc.)
├── versions/       # Installed runtimes by name/version
│   ├── python/3.11.0/
│   └── node/18.16.0/
└── config/
    └── runtimes.json  # Global version config
```

### Key Packages

| Package | Purpose |
|---------|---------|
| `internal/config/` | Paths, version resolution, config file handling |
| `internal/runtime/` | Provider interface, registry, version types |
| `internal/shim/` | Shim lifecycle management |
| `internal/path/` | PATH configuration (platform-specific) |
| `internal/ui/` | Colored output, prompts, verbose/debug logging |
| `internal/tui/` | Table formatting and styles |
| `internal/download/` | File downloads with progress |
| `internal/testutil/` | Shared test utility functions |
| `internal/constants/` | Platform constants |
| `src/cmd/` | CLI commands (one file per command) |
| `src/runtimes/` | Runtime providers (node/, python/) |

---

## Provider System

All runtimes implement the `Provider` interface (`src/internal/runtime/provider.go`, 20 methods total). Providers auto-register via `init()`:

```go
// src/runtimes/node/provider.go
func init() {
    runtime.Register(NewProvider())
}

// src/main.go - blank imports trigger registration
import (
    _ "github.com/dtvem/dtvem/src/runtimes/node"
    _ "github.com/dtvem/dtvem/src/runtimes/python"
)
```

### Key Provider Methods

| Method | Purpose |
|--------|---------|
| `Name()` | Runtime identifier (e.g., "python") |
| `DisplayName()` | Human-readable name (e.g., "Python") |
| `Shims()` | Executable names (e.g., ["python", "pip"]) |
| `ShouldReshimAfter()` | Detect global package installs |
| `Install(version)` | Download and install a version |
| `ExecutablePath(version)` | Path to versioned executable |
| `GlobalPackages(path)` | Detect installed global packages |
| `InstallGlobalPackages()` | Reinstall packages to new version |

### Adding a New Runtime

1. Create `src/runtimes/<name>/provider.go`
2. Implement `runtime.Provider` interface (all 20 methods)
3. Add `init()` function: `runtime.Register(NewProvider())`
4. Import in `src/main.go`: `_ "github.com/dtvem/dtvem/src/runtimes/<name>"`
5. Update `schemas/runtimes.schema.json` enum

The shim mappings are automatically registered via `Shims()`.

---

## Version Resolution

**Priority order:**
1. **Local**: Walk up from `pwd` looking for `.dtvem/runtimes.json` (stops at git root)
2. **Global**: `~/.dtvem/config/runtimes.json`
3. **Error**: No version configured

**Config format** (both local and global):
```json
{
  "python": "3.11.0",
  "node": "18.16.0"
}
```

---

## Key Features

### Automatic Reshim Detection

After `npm install -g` or `pip install`, shims prompt to run `dtvem reshim` to create shims for newly installed executables.

### PATH Fallback

When no dtvem version is configured, shims fall back to system PATH (excluding the shims directory).

### Migration System

The `migrate` command detects existing installations (nvm, pyenv, fnm, system) and offers to:
- Install versions via dtvem
- Preserve global packages (npm packages, pip packages)
- Clean up old installations (automated for version managers, manual instructions for system installs)

**Note**: Configuration file preservation (`.npmrc`, `pip.conf`) is not yet implemented.

---

## Coding Standards

All code follows `GO_STYLEGUIDE.md`. Key points:

- **Naming**: Avoid package/receiver repetition, no "Get" prefix
- **Errors**: Use structured errors, `%w` for wrapping
- **Paths**: Always use `filepath.Join()`, never hardcode `/` or `\`
- **Output**: Use `internal/ui` package for user-facing messages
- **Tests**: Must pass all linters (no special treatment for `*_test.go`)

---

## Testing

### Running Tests

```bash
cd src && go test ./...           # All tests
cd src && go test ./internal/config -v  # Specific package
cd src && go test -cover ./...    # With coverage
```

### Test Coverage

- `internal/config/` - Paths, version resolution, git root detection
- `internal/runtime/` - Registry, provider test harness
- `internal/shim/` - Shim mapping, cache, file operations
- `internal/ui/` - Output formatting functions
- `internal/testutil/` - Test utility functions
- `runtimes/*/` - Provider contract validation
- `cmd/` - Command helpers (migrate, uninstall)

### Provider Test Harness

`internal/runtime/provider_test_harness.go` validates all Provider implementations consistently. Used by node and python providers.

### Import Cycle Avoidance

- Runtime providers import `internal/shim`
- Tests in `internal/shim/` use mock providers (not real ones)
- Real providers tested via the test harness in their own packages

---

## CI/CD

### Build Workflow (`.github/workflows/build.yml`)

1. **golangci-lint** - Linting (errcheck, govet, unused, misspell, etc.)
2. **go-mod** - Verify go.mod/go.sum are tidy
3. **build** - Matrix build/test on Windows, macOS, Linux

PRs get automatic coverage reports posted as comments.

### Release Workflow (`.github/workflows/release.yml`)

Triggered manually via workflow dispatch:

1. **validate** - Check build passed, version format valid
2. **build** - Matrix build for 5 platforms
3. **release** - Create tag, GitHub Release with artifacts
4. **notify** - Post to GitHub Discussions and BlueSky

Version injected at build time; main branch always shows `Version = "dev"`.

---

## Installation Scripts

### One-Command Installers

**Unix:**
```bash
curl -fsSL https://github.com/dtvem/dtvem/releases/latest/download/install.sh | bash
```

**Windows:**
```powershell
irm https://github.com/dtvem/dtvem/releases/latest/download/install.ps1 | iex
```

Features: Auto platform detection, PATH configuration, runs `dtvem init`.

---

## UI Output System

Use `internal/ui` for all user-facing output:

| Function | Purpose |
|----------|---------|
| `Success()` | Green ✓ - completed operations |
| `Error()` | Red ✗ - failures |
| `Warning()` | Yellow ⚠ - non-critical issues |
| `Info()` | Cyan → - informational |
| `Progress()` | Blue → (indented) - operation steps |
| `Header()` | Bold - section titles |
| `Highlight()` | Cyan bold - emphasis |
| `HighlightVersion()` | Magenta bold - version numbers |
| `ActiveVersion()` | Green bold - active/selected versions |
| `DimText()` | Gray/faint - secondary info |
| `Debug()` / `Debugf()` | Debug output (requires `DTVEM_VERBOSE`) |

---

## Important Notes

- **Cross-platform paths**: Use `filepath.Join()`, check `runtime.GOOS`
- **Windows shims**: Must be `.exe` files
- **Shim execution**: Unix uses `syscall.Exec()`; Windows uses `exec.Command()`
- **Version strings**: Strip `v` prefix (e.g., "v22.0.0" → "22.0.0")
- **Registry is global**: Providers auto-register on import via `init()`
- **Verbose mode**: Set `DTVEM_VERBOSE=1` for debug output
