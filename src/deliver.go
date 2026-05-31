package src

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type DeliveryResult string

const (
	DeliveryPasted    DeliveryResult = "pasted"
	DeliveryClipboard DeliveryResult = "clipboard"
)

// function to deliver text to the clipboard or paste it
func DeliverText(text string) (DeliveryResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", fmt.Errorf("transcription was empty")
	}

	if err := copyToClipboard(text); err != nil {
		return "", err
	}

	time.Sleep(75 * time.Millisecond)
	if err := pasteFromClipboard(); err == nil {
		return DeliveryPasted, nil
	}

	return DeliveryClipboard, nil
}

// function to copy text to the clipboard
func copyToClipboard(text string) error {
	switch runtime.GOOS {
	case "linux":
		if commandExists("wl-copy") {
			return writeToCommand(text, "wl-copy")
		}
		if commandExists("xclip") {
			return writeToCommand(text, "xclip", "-selection", "clipboard")
		}
		if commandExists("xsel") {
			return writeToCommand(text, "xsel", "--clipboard", "--input")
		}
	case "darwin":
		if commandExists("pbcopy") {
			return writeToCommand(text, "pbcopy")
		}
	case "windows":
		if commandExists("powershell.exe") {
			return writeToCommand(text, "powershell.exe", "-NoProfile", "-Command", "Set-Clipboard")
		}
		if commandExists("powershell") {
			return writeToCommand(text, "powershell", "-NoProfile", "-Command", "Set-Clipboard")
		}
	}

	return fmt.Errorf("no supported clipboard command found")
}

// function to paste text from the clipboard
func pasteFromClipboard() error {
	switch runtime.GOOS {
	case "linux":
		if commandExists("wtype") {
			return exec.Command("wtype", "-M", "ctrl", "v", "-m", "ctrl").Run()
		}
		if commandExists("xdotool") {
			return exec.Command("xdotool", "key", "ctrl+v").Run()
		}
	case "darwin":
		if commandExists("osascript") {
			return exec.Command("osascript", "-e", `tell application "System Events" to keystroke "v" using command down`).Run()
		}
	case "windows":
		script := `Add-Type -AssemblyName System.Windows.Forms; [System.Windows.Forms.SendKeys]::SendWait("^v")`
		if commandExists("powershell.exe") {
			return exec.Command("powershell.exe", "-NoProfile", "-Command", script).Run()
		}
		if commandExists("powershell") {
			return exec.Command("powershell", "-NoProfile", "-Command", script).Run()
		}
	}

	return fmt.Errorf("no supported paste command found")
}

// function to write input to a command's stdin
func writeToCommand(input string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = bytes.NewBufferString(input)
	return cmd.Run()
}

// function to check whether a particular command exists on the system
func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
