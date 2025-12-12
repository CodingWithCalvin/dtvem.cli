# Release Verification Command

Run comprehensive pre-release verification tests on the dtvem codebase and built binaries.

## Verification Steps

### 1. Build Executables
```bash
go build -v -o dist/dtvem.exe ./src
go build -v -o dist/dtvem-shim.exe ./src/cmd/shim
```

### 2. Run Full Test Suite
```bash
cd src && go test -v ./...
```
- All tests must pass
- Currently 63+ tests across all packages

### 3. Run Linter
```bash
cd src && golangci-lint run ./...
```
- Must report "0 issues"

### 4. Test All Commands

#### Version Command
```bash
./dist/dtvem.exe version
```
Expected: `dtvem version dev` (or release version number)

#### Help Command
```bash
./dist/dtvem.exe help
./dist/dtvem.exe help install
./dist/dtvem.exe help list-all
```
Expected: Shows all available commands and usage information, or detailed help for specific command

#### List Command
```bash
./dist/dtvem.exe list python
./dist/dtvem.exe list node
./dist/dtvem.exe list
```
Expected: Shows installed versions for specified runtime, or all runtimes if no argument

#### List-All Command
```bash
./dist/dtvem.exe list-all python
./dist/dtvem.exe list-all python --filter 3.13
./dist/dtvem.exe list-all node --filter 22
```
Expected: Fetches and displays available versions from official sources, filtered if specified

#### Current Command
```bash
./dist/dtvem.exe current
./dist/dtvem.exe current python
./dist/dtvem.exe current node
```
Expected: Shows currently active runtime versions (global or local)

#### Which Command
```bash
./dist/dtvem.exe which python
./dist/dtvem.exe which node
./dist/dtvem.exe which npm
```
Expected: Shows path to the active runtime executable or shim

#### Where Command
```bash
./dist/dtvem.exe where python 3.11.0
./dist/dtvem.exe where node 20.11.0
```
Expected: Shows installation directory for specific version

#### Completion Command
```bash
./dist/dtvem.exe completion bash
./dist/dtvem.exe completion powershell
./dist/dtvem.exe completion zsh
./dist/dtvem.exe completion fish
```
Expected: Generates shell completion script for specified shell

### 5. Interactive/System-Modifying Commands (Manual Testing)

#### Init Command
```bash
./dist/dtvem.exe init
```
Expected:
- Creates `~/.dtvem` directory structure
- Prompts to add to PATH
- Shows setup success message

#### Install Command
```bash
./dist/dtvem.exe install python 3.13.0
./dist/dtvem.exe install node 22.0.0
./dist/dtvem.exe install  # Bulk install from .dtvem/runtimes.json
```
Expected:
- Downloads and installs specified version
- Shows progress bar
- Creates version directory in `~/.dtvem/versions/`
- Bulk install reads local config and installs all missing versions

#### Uninstall Command
```bash
./dist/dtvem.exe uninstall python 3.11.0
```
Expected:
- Prompts for confirmation (y/N)
- Removes version directory
- Shows success message

#### Global Command
```bash
./dist/dtvem.exe global python 3.13.0
./dist/dtvem.exe global node 22.0.0
```
Expected:
- Sets global version in `~/.dtvem/config/runtimes.json`
- Shows success message

#### Local Command
```bash
./dist/dtvem.exe local python 3.13.0
./dist/dtvem.exe local node 22.0.0
```
Expected:
- Creates `.dtvem/runtimes.json` in current directory
- Sets version for specified runtime
- Shows success message

#### Freeze Command
```bash
./dist/dtvem.exe freeze
```
Expected:
- Reads global runtime versions
- Prompts user to select which runtimes to include
- Creates `.dtvem/runtimes.json` with selected runtimes

#### Migrate Command
```bash
./dist/dtvem.exe migrate
```
Expected:
- Detects existing runtime installations (nvm, pyenv, system)
- Prompts to migrate each detected version
- Installs via dtvem
- Preserves global packages
- Optionally cleans up old installations

#### Reshim Command
```bash
./dist/dtvem.exe reshim
```
Expected:
- Scans installed runtime versions
- Creates/updates shim executables in `~/.dtvem/shims/`
- Shows success message with count of shims created

## Success Criteria

✅ All builds complete without errors
✅ All unit tests pass
✅ Linter reports 0 issues
✅ All major commands execute successfully
✅ Network-dependent commands (list-all) successfully fetch data
✅ No runtime panics or crashes

## Notes

- The `version` command will show "dev" until a release is created
- `list` commands will only show installed runtimes
- `list-all` requires internet connection to fetch available versions
- Some commands (install, init) may require elevated permissions or modify the system

## Supported Runtimes

### Currently Implemented
- ✅ **Python** (`python`) - Multiple versions with pip package management
- ✅ **Node.js** (`node`) - Multiple versions with npm package management

### Runtime Testing Matrix

For each supported runtime, test the following commands:

#### Python Runtime
```bash
./dist/dtvem.exe list python
./dist/dtvem.exe list-all python
./dist/dtvem.exe list-all python --filter 3.13
./dist/dtvem.exe current python
./dist/dtvem.exe which python
./dist/dtvem.exe which pip
./dist/dtvem.exe which pip3
./dist/dtvem.exe where python 3.11.0
```

#### Node.js Runtime
```bash
./dist/dtvem.exe list node
./dist/dtvem.exe list-all node
./dist/dtvem.exe list-all node --filter 22
./dist/dtvem.exe current node
./dist/dtvem.exe which node
./dist/dtvem.exe which npm
./dist/dtvem.exe which npx
./dist/dtvem.exe where node 20.11.0
```

### Runtime Provider Contract

Each runtime provider must implement:
- Name() - Runtime identifier
- DisplayName() - User-friendly name
- Shims() - List of shim executables to create
- ListInstalled() - List installed versions
- ListAvailable() - List available versions from official sources
- Install(version) - Download and install a version
- Uninstall(version) - Remove an installed version
- IsInstalled(version) - Check if version is installed
- ExecutablePath(version) - Get path to runtime executable
- DetectInstalled() - Detect existing installations (for migration)
- GlobalVersion() / SetGlobalVersion() - Manage global versions
- LocalVersion() / SetLocalVersion() - Manage local versions
- CurrentVersion() - Get currently active version
- GlobalPackages(version) - List globally installed packages
- ManualPackageInstallCommand(packages) - Get command to reinstall packages
- InstallPath() - Base installation directory
- ShouldReshimAfter(shimName, args) - Detect when reshim is needed

## Command Coverage

### Non-Interactive Commands (Automated Testing)
- ✅ **version** - Show dtvem version
- ✅ **help** - Show help information
- ✅ **list** - List installed versions
- ✅ **list-all** - List available versions (requires internet)
- ✅ **current** - Show active versions
- ✅ **which** - Show path to command
- ✅ **where** - Show installation directory
- ✅ **completion** - Generate shell completion scripts

### Interactive/System-Modifying Commands (Manual Testing)
- 🔧 **init** - Initialize dtvem (creates directories, modifies PATH)
- 🔧 **install** - Install runtime version (downloads, installs)
- 🔧 **uninstall** - Uninstall runtime version (prompts for confirmation)
- 🔧 **global** - Set global version (modifies `~/.dtvem/config/runtimes.json`)
- 🔧 **local** - Set local version (creates `.dtvem/runtimes.json`)
- 🔧 **freeze** - Create runtime config from global versions
- 🔧 **migrate** - Migrate from other version managers (interactive)
- 🔧 **reshim** - Regenerate shims

### Total Commands: 16
