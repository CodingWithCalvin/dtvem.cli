# Commit Message Convention

This project follows the [Conventional Commits](https://www.conventionalcommits.org/) specification for commit messages. This helps us:

- Automatically generate changelogs
- Easily navigate through git history
- Trigger automated versioning and releases
- Make collaboration easier

## Commit Message Format

Each commit message consists of a **header**, an optional **body**, and an optional **footer**:

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Header (Required)

The header is mandatory and must conform to this format:

```
<type>(<scope>): <subject>
```

- **type**: The type of change (see below)
- **scope**: Optional. The module/component affected (e.g., `node`, `python`, `cli`, `install`)
- **subject**: A short description of the change (lowercase, no period at end, max 72 chars)

### Types

Must be one of the following:

| Type | Description | Example |
|------|-------------|---------|
| **feat** | A new feature | `feat(node): add support for Node.js 22.x` |
| **fix** | A bug fix | `fix(python): correct version detection on Windows` |
| **docs** | Documentation only changes | `docs: update installation instructions` |
| **style** | Code style changes (formatting, etc.) | `style: run gofmt on all files` |
| **refactor** | Code refactoring (no bug fix or feature) | `refactor(shim): simplify path resolution logic` |
| **perf** | Performance improvements | `perf(install): parallelize downloads` |
| **test** | Adding or updating tests | `test(node): add detection tests for nvm` |
| **build** | Build system or dependency changes | `build: update go.mod dependencies` |
| **ci** | CI/CD configuration changes | `ci: add coverage reporting workflow` |
| **chore** | Other changes (maintenance, etc.) | `chore: update .gitignore` |
| **revert** | Revert a previous commit | `revert: feat(node): add Node.js 22.x support` |

### Scope (Optional)

The scope should be the name of the affected module or component:

- `node` - Node.js runtime provider
- `python` - Python runtime provider
- `cli` - CLI commands
- `install` - Installation logic
- `migrate` - Migration functionality
- `shim` - Shim system
- `config` - Configuration handling
- `path` - PATH management
- `ui` - User interface / output
- `test` - Test infrastructure
- `docs` - Documentation

### Subject (Required)

The subject contains a succinct description of the change:

- Use the imperative, present tense: "add" not "added" nor "adds"
- Start with lowercase (but uppercase abbreviations like PR, API, CLI, URL are allowed)
- No period (.) at the end
- Maximum 72 characters

**Good examples:**
- `fix(python): handle missing pip.exe on Windows`
- `feat(cli): add --yes flag to install command`
- `docs: add troubleshooting section to README`
- `chore(ci): remove PR coverage comments`
- `feat: add API endpoint for version lookup`

**Bad examples:**
- `Fixed bug` (missing type, not descriptive)
- `feat(node): Added support for nvm.` (wrong tense, period at end)
- `Update code` (missing type, not descriptive)

### Body (Optional)

The body should include the motivation for the change and contrast with previous behavior.

- Use the imperative, present tense: "change" not "changed" nor "changes"
- Wrap at 100 characters
- Include the context of the change

### Footer (Optional)

The footer should contain:

- **Breaking Changes**: Start with `BREAKING CHANGE:` followed by description
- **Issue references**: `Fixes #123`, `Closes #456`, `Relates to #789`

## Examples

### Simple commit (no scope)

```
docs: fix typo in README
```

### Feature with scope

```
feat(python): add support for pyenv-win detection

Detect Python installations managed by pyenv-win on Windows.
Checks both %USERPROFILE%\.pyenv\pyenv-win\versions and legacy locations.

Fixes #42
```

### Bug fix with breaking change

```
fix(cli)!: change global command behavior

BREAKING CHANGE: The `global` command now validates versions before setting.
This may cause existing scripts to fail if they set invalid versions.

Previously, invalid versions were accepted and only failed at runtime.
Now, the command will exit with an error if the version is not installed.

Fixes #156
```

### Multiple issue references

```
feat(node): add version caching to improve performance

Cache Node.js version list for 24 hours to reduce network calls.
Significantly improves `list-all node` performance.

Fixes #89
Relates to #72, #98
```

### Revert commit

```
revert: feat(ruby): add Ruby runtime support

This reverts commit abc123def456.

Reverting due to Windows compatibility issues that need more investigation.

Relates to #234
```

## Pull Requests and Squash Merges

This project uses **squash merges** for all pull requests. This means:

- All commits in your PR are combined into a single commit on merge
- **The PR title becomes the commit message** on the main branch
- Your PR title must follow the conventional commit format

When creating a PR, ensure your title follows the format:
```
<type>(<scope>): <subject>
```

For example:
- `feat(node): add support for Node.js 22.x`
- `fix(shim): handle API errors on Windows`
- `chore(ci): update PR linting workflow`

The PR title is validated automatically - if it doesn't follow the convention, the CI check will fail.

## Validation

Both PR titles and individual commits are automatically validated using [commitlint](https://commitlint.js.org/). If they don't follow this convention, the CI check will fail.

### Tips for Success

1. **Write clear, descriptive subjects** - Future you will thank you!
2. **Use the body for context** - Explain *why* not *what* (code shows what)
3. **Reference issues** - Link to GitHub issues for traceability
4. **One logical change per commit** - Makes history easier to navigate
5. **Test your changes** - Every commit should leave the code in a working state

## Fixing Non-Conforming Commits

If your commit messages don't follow the convention:

### For the last commit:
```bash
git commit --amend
# Edit the commit message to follow the convention
git push --force-with-lease
```

### For multiple commits:
```bash
# Rebase and reword commits
git rebase -i HEAD~3  # Replace 3 with number of commits to edit
# Mark commits as 'reword' in the editor
# Update each commit message to follow the convention
git push --force-with-lease
```

### Interactive rebase cheatsheet:
- `pick` - Keep commit as-is
- `reword` - Keep changes but edit commit message
- `squash` - Combine with previous commit
- `drop` - Remove commit entirely

## Tools

### Commit Message Template

You can add a commit message template to help remember the format:

```bash
# Create template file
cat > ~/.gitmessage << 'EOF'
# <type>(<scope>): <subject>
#
# <body>
#
# <footer>
#
# Types: feat, fix, docs, style, refactor, perf, test, build, ci, chore, revert
# Scope: node, python, cli, install, migrate, shim, config, path, ui, test, docs
#
# Subject: imperative, lowercase, no period, max 72 chars
# Body: motivation for change, max 100 chars per line
# Footer: breaking changes, issue references
EOF

# Configure git to use it
git config --global commit.template ~/.gitmessage
```

### Editor Setup

**VS Code**: Install the "Conventional Commits" extension:
```bash
code --install-extension vivaxy.vscode-conventional-commits
```

**IntelliJ/GoLand**: Install the "Git Commit Template" plugin

## Resources

- [Conventional Commits Specification](https://www.conventionalcommits.org/)
- [commitlint Documentation](https://commitlint.js.org/)
- [Semantic Versioning](https://semver.org/)

## Questions?

If you have questions about commit message formatting, feel free to ask in your pull request or open a discussion!
