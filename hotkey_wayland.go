//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"
)

func setupHotkey(toggleFn func()) error {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("not wayland")
	}

	os.Remove(pipePath)
	if err := syscall.Mkfifo(pipePath, 0600); err != nil {
		return fmt.Errorf("failed to create pipe: %v", err)
	}

	fmt.Println("Wayland mode: add this to your Sway config:")
	fmt.Printf("  bindsym Ctrl+Space exec echo toggle > %s\n\n", pipePath)

	go func() {
		for {
			f, err := os.Open(pipePath)
			if err != nil {
				continue
			}
			buf := make([]byte, 16)
			n, _ := f.Read(buf)
			f.Close()
			if string(buf[:n]) == "toggle\n" {
				toggleFn()
			}
		}
	}()

	return nil
}
