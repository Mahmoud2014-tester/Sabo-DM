//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func scheduleWindowsSelfUpdate(staged string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	staged, err = filepath.Abs(staged)
	if err != nil {
		return err
	}
	exe, err = filepath.Abs(exe)
	if err != nil {
		return err
	}
	if _, err := os.Stat(staged); err != nil {
		return fmt.Errorf("staged update is missing: %w", err)
	}
	script := filepath.Join(configDir(), "apply-sabo-update.bat")
	q := func(v string) string { return strings.ReplaceAll(v, `"`, `""`) }
	body := fmt.Sprintf(`@echo off
set "SABO_PID=%d"
:wait_for_sabo
tasklist /FI "PID eq %%SABO_PID%%" /NH | find "%%SABO_PID%%" >nul
if not errorlevel 1 (timeout /t 1 /nobreak >nul & goto wait_for_sabo)
copy /Y "%s" "%s" >nul
if errorlevel 1 exit /b 2
start "" "%s"
del "%%~f0"
`, os.Getpid(), q(staged), q(exe), q(exe))
	if err := os.WriteFile(script, []byte(body), 0600); err != nil {
		return err
	}
	cmd := exec.Command("cmd.exe", "/C", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(script)
		return err
	}
	_ = cmd.Process.Release()
	return nil
}
