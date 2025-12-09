# Claude Code Instructions for dtvem

## Deployment

After building, always deploy both executables to the user's installation directory:

```bash
cp dist/dtvem.exe ~/.dtvem/bin/dtvem.exe
cp dist/shim.exe ~/.dtvem/bin/dtvem-shim.exe
~/.dtvem/bin/dtvem.exe reshim
```

The `reshim` command must be run after deploying the shim to regenerate all runtime shims (npm, node, python, etc.) with the updated binary.
