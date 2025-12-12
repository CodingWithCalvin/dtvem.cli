# Issue Analysis Command

Perform a comprehensive analysis of all GitHub issues against the current codebase to identify implemented, duplicate, and still-relevant issues.

## Instructions

### Phase 1: Gather Issues

1. Fetch all OPEN issues from the repository:
   ```bash
   gh issue list --state open --limit 200 --json number,title,body
   ```

2. Fetch all CLOSED issues for duplicate detection:
   ```bash
   gh issue list --state closed --limit 200 --json number,title
   ```

### Phase 2: Analyze Codebase

For each open issue, determine its status by examining the actual codebase:

1. **Use the Explore agent** to search for implementations:
   - Search for relevant files, functions, and patterns mentioned in the issue
   - Check src/cmd/ for command implementations
   - Check src/runtimes/ for provider implementations
   - Check src/internal/ for core functionality
   - Check .github/workflows/ for CI/CD features
   - Check package.json, .golangci.yml for tooling

2. **Classify each issue** as one of:
   - **IMPLEMENTED**: Feature fully exists in codebase → recommend closing
   - **PARTIALLY_IMPLEMENTED**: Some work done, some remaining → recommend updating issue body
   - **RELEVANT**: Not yet implemented, still needed → keep open as-is
   - **DUPLICATE**: Covered by another issue (open or closed) → recommend closing as duplicate

### Phase 3: Present Findings for Approval

**IMPORTANT: Do not modify or close any issues without explicit user approval.**

Present a summary table of recommended actions:

```markdown
## Recommended Actions

### Issues to Close (Implemented)
| # | Title | Implementation Location |
|---|-------|------------------------|

### Issues to Close (Duplicate)
| # | Title | Duplicate Of |
|---|-------|--------------|

### Issues to Update (Partially Implemented)
| # | Title | What's Done | What Remains |
|---|-------|-------------|--------------|

### Issues to Keep As-Is (Relevant)
| # | Title | Notes |
|---|-------|-------|
```

Then ask: **"Would you like me to proceed with these changes? You can also specify individual issues to skip."**

### Phase 4: Execute Approved Actions

Only after user approval, execute the changes:

#### For IMPLEMENTED issues (user approved):
```bash
gh issue close <number> --reason completed --comment "Implemented in <file/location>. <brief description of implementation>"
```

#### For DUPLICATE issues (user approved):
```bash
gh issue close <number> --reason "not planned" --comment "Closing as duplicate of #<other> which covers the same scope."
```

#### For PARTIALLY_IMPLEMENTED issues (user approved):
Update the issue body to show only remaining work:
1. Add "## Current State" section listing what's already done (with ✅)
2. Add "## Remaining Work" section with checkbox items for what's left
3. Update acceptance criteria to reflect only incomplete items
4. Use `gh issue edit <number> --body-file <temp_file>` to update

### Phase 5: Generate Report

Create/update `.claude/ISSUE_ANALYSIS.md` with:

```markdown
# GitHub Issue Analysis Report

> **Generated:** <date>
> **Open Issues:** <count>

## Executive Summary

| Status | Count | Description |
|--------|-------|-------------|
| **OPEN** | X | Issues requiring work |
| **CLOSED (Implemented)** | X | Features completed |
| **CLOSED (Duplicate)** | X | Consolidated into other issues |

## Open Issues by Category

### <Category Name>
| # | Title | Notes |
|---|-------|-------|
| X | <title> | <remaining work summary> |

## Recently Closed Issues

### Closed as Implemented
| # | Title | Reason |
|---|-------|--------|

### Closed as Duplicate
| # | Title | Duplicate Of |
|---|-------|--------------|

## Priority Recommendations

### High Priority
- Security issues
- Core functionality gaps

### Medium Priority
- Feature requests
- Testing gaps

### Lower Priority
- Enhancements
- Nice-to-haves
```

## Key Analysis Patterns

### Checking if a feature is implemented:

1. **Commands**: Look in `src/cmd/<command>.go`
2. **Runtime providers**: Look in `src/runtimes/<runtime>/provider.go`
3. **CI features**: Look in `.github/workflows/*.yml`
4. **Configuration**: Look in `src/internal/config/`
5. **Shim behavior**: Look in `src/cmd/shim/main.go`

### Common duplicate patterns:

- Parent/child issues (e.g., "add shell completion" vs "add Bash completion")
- Umbrella issues (e.g., "add Ruby, Go, Rust" vs individual provider issues)
- Overlapping scope (e.g., two issues both addressing "error handling")

### Updating partially implemented issues:

Structure the updated body as:
```markdown
<Brief description of what this issue accomplishes>

## Current State

- ✅ <completed item 1>
- ✅ <completed item 2>
- ❌ <not yet done item>

## Remaining Work

### <Category>
- [ ] <specific task>
- [ ] <specific task>

## Acceptance Criteria

- [ ] <criterion that's not yet met>
- [ ] ~~<criterion already met>~~ (already done)
```

## Output

After completing the analysis:
1. Report how many issues were closed (implemented + duplicates)
2. Report how many issues were updated (partially implemented)
3. Report the new open issue count
4. Provide the path to the updated analysis document
