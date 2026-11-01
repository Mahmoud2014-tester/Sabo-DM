@echo off
setlocal
cd /d "%~dp0"
title Sabo DM Diagnostics

echo Starting Sabo DM in diagnostic mode...
echo Keep this window open. Any startup errors should appear here.
echo A log file named sabo-dm-dev.log will also be written beside the application.
echo.
if exist "Sabo DM-Diagnostic.exe" (
  "Sabo DM-Diagnostic.exe"
  set "APP_EXIT=%ERRORLEVEL%"
  echo.
  echo Sabo DM diagnostic process exited with code %APP_EXIT%.
  if exist sabo-dm-dev.log (
    echo Log file: %CD%\sabo-dm-dev.log
    echo.
    type sabo-dm-dev.log
  )
  pause
  exit /b %APP_EXIT%
)
if exist "Sabo DM.exe" (
  echo No diagnostic executable found. Run build.bat to create it.
) else (
  echo No Sabo DM executable found. Run build.bat first.
)
pause
exit /b 1
