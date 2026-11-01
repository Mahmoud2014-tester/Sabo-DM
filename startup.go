package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Starting with Windows uses the per-user "Run" registry key (no admin rights needed).
// The app is started with --background so it goes straight to the tray.

const (
	runKey  = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	runName = "Sabo DM"
)

func autostartEnabled() bool {
	out, err := hidden(exec.Command("reg", "query", runKey, "/v", runName)).Output()
	return err == nil && strings.Contains(string(out), runName)
}

func setAutostart(on bool) error {
	if on {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		return hidden(exec.Command("reg", "add", runKey, "/v", runName, "/t", "REG_SZ",
			"/d", fmt.Sprintf(`"%s" --background`, exe), "/f")).Run()
	}
	err := hidden(exec.Command("reg", "delete", runKey, "/v", runName, "/f")).Run()
	if err != nil && autostartEnabled() {
		return err // a missing value is fine; a value that could not be deleted is not
	}
	return nil
}
