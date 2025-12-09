# Claude Code Instructions for dtvem

## Commits

All commits must be authored by `calvin@codingwithcalvin.net`. Do not add co-authors (including Claude) to commits.

## Deployment

After building, always deploy both executables to the user's installation directory:

```bash
cp dist/dtvem.exe ~/.dtvem/bin/dtvem.exe
cp dist/shim.exe ~/.dtvem/bin/dtvem-shim.exe
~/.dtvem/bin/dtvem.exe reshim
```

The `reshim` command must be run after deploying the shim to regenerate all runtime shims (npm, node, python, etc.) with the updated binary.
