//go:build !windows

package main

import "errors"

func scheduleWindowsSelfUpdate(staged string) error {
	return errors.New("automatic self-update is supported only on Windows")
}
