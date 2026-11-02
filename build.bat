@echo off
setlocal EnableExtensions
cd /d "%~dp0"
title Sabo DM Builder

echo ========================================
echo       Sabo DM - Windows Build
echo ========================================
echo.

where go >nul 2>nul
if errorlevel 1 (
  echo ERROR: Go is not installed or is not on PATH.
  echo Install Go 1.23 or newer from https://go.dev/dl/
  echo Then close and reopen this window and run build.bat again.
  goto :fail
)

echo [1/6] Checking Go installation...
go version
if errorlevel 1 goto :fail

echo.
echo [2/6] Downloading and resolving project dependencies...
set GOPROXY=https://proxy.golang.org,direct
go mod tidy
if errorlevel 1 (
  echo ERROR: Could not download Go dependencies.
  echo Check your internet connection, proxy/firewall settings, and try again.
  goto :fail
)

echo.
echo [3/6] Running project tests...
go test ./...
if errorlevel 1 (
  echo ERROR: One or more project tests failed.
  goto :fail
)

echo.
echo [4/6] Installing Windows resource tool...
for /f "delims=" %%i in ('go env GOPATH') do set "GP=%%i"
if not exist "%GP%\bin\rsrc.exe" go install github.com/akavel/rsrc@latest
if errorlevel 1 goto :fail
"%GP%\bin\rsrc.exe" -manifest Sabo-DM.manifest -o rsrc.syso
if errorlevel 1 goto :fail

echo.
echo [5/6] Building normal GUI application...
go build -trimpath -ldflags="-s -w -H windowsgui -X main.AppVersion=1.3.13-dev" -o Sabo-DM.exe .
if errorlevel 1 goto :fail

echo.
echo [6/6] Building diagnostic version (console + log)...
go build -trimpath -tags dev -ldflags="-X main.AppVersion=1.3.13-dev" -o Sabo-DM-Diagnostic.exe .
if errorlevel 1 goto :fail

echo.
echo BUILD SUCCESSFUL.
echo Files created:
echo   Sabo-DM.exe              - normal GUI version
echo   Sabo-DM-Diagnostic.exe   - diagnostic version; keep its console open
echo.
echo If the normal app does not open, run Sabo-DM-Diagnostic.exe and send sabo-dm-dev.log plus any console error.
pause
exit /b 0

:fail
echo.
echo BUILD FAILED. The error above explains what needs fixing.
echo If this is a dependency download error, connect to the internet and retry.
pause
exit /b 1
