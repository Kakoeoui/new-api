@echo off
setlocal
cd /d "%~dp0"
if errorlevel 1 goto failed
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\start-local.ps1" %*
if errorlevel 1 goto failed
exit /b 0

:failed
echo Local development did not start. See the message above.
pause
exit /b 1
