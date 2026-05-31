package src

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
)

//go:embed gui.py
var guiScript string

var (
	guiCmd   *exec.Cmd
	guiStdin io.WriteCloser
	guiMu    sync.Mutex
)

// StartGUI launches the visual feedback floating pill
func StartGUI() error {
	guiMu.Lock()
	defer guiMu.Unlock()

	if guiCmd != nil {
		return nil
	}

	scriptPath := "/tmp/dictate_gui.py"
	err := os.WriteFile(scriptPath, []byte(guiScript), 0755)
	if err != nil {
		return fmt.Errorf("failed to write GUI script: %w", err)
	}

	cmd := exec.Command("python3", scriptPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to open GUI stdin pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start GUI: %w", err)
	}

	guiCmd = cmd
	guiStdin = stdin

	go func() {
		_ = cmd.Wait()
		guiMu.Lock()
		guiCmd = nil
		guiStdin = nil
		guiMu.Unlock()
	}()

	return nil
}

// SetGUIState updates the state of the floating status pill
func SetGUIState(state string, detail string) {
	guiMu.Lock()
	defer guiMu.Unlock()

	if guiStdin == nil {
		// Try to start GUI on demand
		guiMu.Unlock()
		_ = StartGUI()
		guiMu.Lock()
		if guiStdin == nil {
			return
		}
	}

	var msg string
	if detail != "" {
		msg = fmt.Sprintf("state %s %s\n", state, detail)
	} else {
		msg = fmt.Sprintf("state %s\n", state)
	}

	_, _ = guiStdin.Write([]byte(msg))
}

// SendGUIVolume sends real-time volume input to animate the waveform
func SendGUIVolume(vol float64) {
	guiMu.Lock()
	defer guiMu.Unlock()

	if guiStdin == nil {
		return
	}

	msg := fmt.Sprintf("volume %.3f\n", vol)
	_, _ = guiStdin.Write([]byte(msg))
}

// CloseGUI terminates the status overlay process
func CloseGUI() {
	guiMu.Lock()
	defer guiMu.Unlock()

	if guiStdin != nil {
		_, _ = guiStdin.Write([]byte("quit\n"))
		_ = guiStdin.Close()
		guiStdin = nil
	}
	if guiCmd != nil {
		_ = guiCmd.Process.Kill()
		guiCmd = nil
	}
}
