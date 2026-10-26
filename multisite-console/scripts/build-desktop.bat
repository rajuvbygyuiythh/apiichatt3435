@echo off
REM Builds the Windows desktop app (Multi-Site-Console.exe) with Wails.
setlocal
cd /d "%~dp0.."

echo [1/4] Syncing the dashboard into the desktop shell...
call scripts\sync-ui.bat
if errorlevel 1 goto :fail

echo [2/4] Resolving desktop modules...
pushd desktop
go mod tidy
if errorlevel 1 goto :fail

echo [3/4] Building (wails build)...
wails build -clean -platform windows/amd64
if errorlevel 1 goto :fail
popd

echo [4/4] Copying sites + scripts next to the executable...
if not exist build\bin mkdir build\bin
xcopy /E /I /Y sites build\bin\sites >nul
copy /Y scripts\run-web.bat build\bin\ >nul 2>nul

echo.
echo Built: build\bin\Multi-Site-Console.exe
echo Run it, or double-click it in Explorer.
exit /b 0

:fail
echo Desktop build failed. You can still use scripts\run-web.bat
pause
exit /b 1
