//go:build !linux

package main

import (
	"fmt"

	"golang.design/x/hotkey"
)

func setupHotkey(toggleFn func()) error {
	hk := hotkey.New([]hotkey.Modifier{hotkey.ModCtrl}, hotkey.KeySpace)
	if err := hk.Register(); err != nil {
		return fmt.Errorf("failed to register hotkey: %v", err)
	}

	fmt.Println("Hotkey registered: Ctrl+Space")

	go func() {
		for range hk.Keydown() {
			toggleFn()
		}
	}()

	return nil
}
