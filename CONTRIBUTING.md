# Contributing to dtvem

First off, thank you for considering contributing to dtvem! It's people like you that make dtvem such a great tool.

## Code of Conduct

This project and everyone participating in it is governed by the [Contributor Covenant Code of Conduct](CODE_OF_CONDUCT.md). By participating, you are expected to uphold this code. Please report unacceptable behavior by opening a GitHub issue or contacting the project maintainers.

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When creating a bug report, include as many details as possible:

- **Use a clear and descriptive title**
- **Describe the exact steps to reproduce the problem**
- **Provide specific examples** (commands run, expected vs actual behavior)
- **Describe the environment** (OS, Go version, dtvem version)
- **Include logs or error messages** if applicable

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion:

- **Use a clear and descriptive title**
- **Provide a detailed description** of the suggested enhancement
- **Explain why this enhancement would be useful** to most dtvem users
- **List any alternatives** you've considered

### Pull Requests

1. Fork the repo and create your branch from `main`
2. If you've added code that should be tested, add tests
3. Ensure your code follows the existing style
4. **Use a conventional commit format for your PR title** (e.g., `feat(node): add version caching`)
5. Submit your pull request!

**Note:** We use squash merges, so your PR title becomes the commit message on main. Make sure it follows the [commit convention](docs/COMMIT_CONVENTION.md).

## Development Setup

### Prerequisites

- Go 1.23 or higher
- Git
- A code editor (VS Code, GoLand, etc.)

### Building

```bash
# Clone your fork
git clone https://github.com/YOUR_USERNAME/dtvem.git
cd dtvem

# Build the main executable
go build -o dist/dtvem.exe ./src

# Build the shim executable
go build -o dist/dtvem-shim.exe ./src/cmd/shim

# Run the executable
./dist/dtvem.exe help
```

### Running Tests

```bash
# Run all tests (when implemented)
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests for a specific package
go test ./src/internal/runtime/
```

## Coding Standards

### Go Style Guide

This project follows the [Google Go Style Guide](https://google.github.io/styleguide/go/) as documented in `.claude/GO_STYLEGUIDE.md`.

**Required before submitting:**
- Run `golangci-lint run` to check formatting, style, and correctness (includes gofmt, govet, and 40+ linters)
- Ensure `go mod tidy` has been run

**Code quality tools:**
- **golangci-lint v1.62+**: Comprehensive linting (includes gofmt, goimports, govet, staticcheck, revive, and 40+ more linters)
- All checks run automatically on every PR

### Installing golangci-lint

**macOS:**
```bash
# Using Homebrew (recommended)
brew install golangci-lint

# Update to latest version
brew upgrade golangci-lint
```

**Linux:**
```bash
# Using Homebrew
brew install golangci-lint

# Or using install script
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin

# Or using binary (replace VERSION and OS/ARCH)
curl -sSfL https://github.com/golangci/golangci-lint/releases/download/v1.55.2/golangci-lint-1.55.2-linux-amd64.tar.gz | tar xz -C $(go env GOPATH)/bin --strip-components=1 golangci-lint-1.55.2-linux-amd64/golangci-lint
```

**Windows:**
```powershell
# Using WinGet (recommended - pre-installed on Windows 10/11)
winget install golangci.golangci-lint

# Update to latest version
winget upgrade golangci.golangci-lint

# Using Scoop
scoop install golangci-lint

# Using Chocolatey
choco install golangci-lint

# Or download binary from releases page
# https://github.com/golangci/golangci-lint/releases
```

**Any Platform (using Go):**
```bash
# Install latest version (v1.62+ required for Go 1.23)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Or install specific version
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2

# Verify installation (should be v1.62+)
golangci-lint --version
```

**Note:** If using `go install`, make sure `$(go env GOPATH)/bin` is in your PATH.

### Running Linters Locally

```bash
# Run all linters (what CI runs) - includes gofmt, govet, and 40+ more
golangci-lint run

# Auto-fix issues where possible (including formatting)
golangci-lint run --fix

# Run all checks before committing
golangci-lint run && go mod tidy && go test ./src/...

# Run specific linters only
golangci-lint run --disable-all --enable=errcheck,govet,gofmt
```

**Note:** `golangci-lint` automatically includes `gofmt`, `goimports`, and `govet`, so you don't need to run them separately!

### Code Organization

```
src/
├── cmd/              # CLI commands (one file per command)
├── internal/         # Internal packages (not importable by other projects)
│   ├── config/      # Configuration and path management
│   ├── runtime/     # Core plugin system
│   └── shim/        # Shim management
└── runtimes/        # Runtime provider implementations
    ├── python/
    └── node/
```

### Commit Messages

This project follows the [Conventional Commits](https://www.conventionalcommits.org/) specification. All commits must follow this format:

```
<type>(<scope>): <subject>
```

**Examples:**
```
feat(python): add Python 3.12 detection support
fix(node): correct version detection on Windows
docs: update installation instructions
test(migrate): add tests for package preservation
```

**Common types:** `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`

**Common scopes:** `node`, `python`, `cli`, `install`, `migrate`, `shim`, `config`, `path`, `ui`, `test`, `docs`

For detailed guidelines and examples, see [Commit Convention Guide](docs/COMMIT_CONVENTION.md).

**Note:** PR titles are automatically validated for conventional commit compliance. Non-conforming titles will fail CI checks. Since we use squash merges, your PR title becomes the final commit message.

## Adding a New Runtime Provider

To add support for a new runtime (e.g., Ruby):

1. Create `src/runtimes/ruby/provider.go`
2. Implement the `runtime.Provider` interface:
   ```go
   type Provider struct {}

   func (p *Provider) Name() string { return "ruby" }
   func (p *Provider) DisplayName() string { return "Ruby" }
   // ... implement all other methods
   ```
3. Register the provider in the `init()` function:
   ```go
   func init() {
       runtime.Register(NewProvider())
   }
   ```
4. Import the provider in `src/main.go`:
   ```go
   _ "github.com/dtvem/dtvem/src/runtimes/ruby"
   ```
5. Implement the `Shims()` method to define which executables this runtime provides
6. Implement the `ShouldReshimAfter()` method for automatic reshim detection
7. Add tests using the provider contract harness
8. Update README.md to reflect the new runtime support

## Testing Guidelines

When writing tests:

- Place test files next to the code they test (`provider_test.go` next to `provider.go`)
- Use table-driven tests where appropriate
- Test both success and error cases
- Mock external dependencies (network calls, file system)
- Use descriptive test names: `TestNodeProvider_DetectInstalled_WithNVM`

Example test structure:
```go
func TestNodeProvider_DetectInstalled(t *testing.T) {
    tests := []struct {
        name    string
        setup   func()
        want    int
        wantErr bool
    }{
        {
            name: "detects system node",
            setup: func() { /* setup test environment */ },
            want: 1,
            wantErr: false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

## Documentation

- Update README.md when adding new features
- Close related GitHub Issues when completing work
- Add inline comments for complex logic
- Update command help text when modifying commands
- Add examples for new functionality

## Questions?

Feel free to open an issue with the label `question` if you need help or clarification on anything!

## Recognition

Contributors will be recognized in the README.md and release notes. Thank you for making dtvem better!

---

Happy Contributing! 🚀
