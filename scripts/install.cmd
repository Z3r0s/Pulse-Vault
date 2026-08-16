@echo off
REM Double-click from a clone: installs CLI + GUI (builds if no GitHub Release).
REM DNSPulse — https://dnspulse.org
setlocal
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0install.ps1" -WithGui %*
if errorlevel 1 pause
endlocal
