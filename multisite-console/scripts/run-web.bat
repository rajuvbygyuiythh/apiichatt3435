@echo off
REM Starts the console in web mode (no Wails needed) and opens the browser.
setlocal
cd /d "%~dp0.."
if not exist data mkdir data
start "" http://127.0.0.1:8090
go run ./cmd/console -addr 127.0.0.1:8090 -data data -sites sites
