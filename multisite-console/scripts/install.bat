@echo off
REM ============================================================
REM  Multi-Site Console - one-time setup (Windows)
REM  Installs the Go toolchain check, the Wails CLI and modules.
REM ============================================================
setlocal

where go >nul 2>nul
if errorlevel 1 (
  echo [1/3] Go was not found.
  echo       Install it from https://go.dev/dl/ ^(go1.24 or newer^) and run this file again.
  pause
  exit /b 1
)

echo [1/3] Go found:
go version

echo [2/3] Downloading console modules...
go mod download
if errorlevel 1 goto :fail

echo [3/3] Installing the Wails CLI ^(desktop build only^)...
go install github.com/wailsapp/wails/v2/cmd/wails@latest
if errorlevel 1 (
  echo       Wails CLI install failed. The web build still works: scripts\run-web.bat
) else (
  echo       Wails installed. Run: scripts\build-desktop.bat
)

echo.
echo Done. Start the console with: scripts\run-web.bat
exit /b 0

:fail
echo Setup failed. Check the network connection and try again.
pause
exit /b 1
