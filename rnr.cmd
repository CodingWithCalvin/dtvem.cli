@echo off
setlocal

:: Detect architecture
if "%PROCESSOR_ARCHITECTURE%"=="ARM64" (
  set "ARCH=arm64"
) else (
  set "ARCH=amd64"
)

set "BINARY=%~dp0.rnr\bin\rnr-windows-%ARCH%.exe"

if not exist "%BINARY%" (
  echo Error: rnr is not configured for windows-%ARCH%. >&2
  echo Run 'rnr init --add-platform windows-%ARCH%' to add support. >&2
  exit /b 1
)

"%BINARY%" %*
