@echo off
REM Copies cmd\console\ui into desktop\frontend\dist (the Wails asset folder).
setlocal
cd /d "%~dp0.."
if not exist desktop\frontend\dist mkdir desktop\frontend\dist
xcopy /E /I /Y cmd\console\ui\* desktop\frontend\dist\ >nul
echo UI synced to desktop\frontend\dist
