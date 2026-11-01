//go:build dev

package main

import (
	"log"
	"os"
)

func init() {
	if f, err := os.OpenFile("sabo-dm-dev.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644); err == nil {
		log.SetOutput(f)
	}
}

func debugf(f string, a ...any) { log.Printf("[dev] "+f, a...) }
