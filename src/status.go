package src

import (
	"fmt"
	"os/exec"
)

const appName = "Dictate"

type Status string

const (
	StatusRecording    Status = "Recording..."
	StatusTranscribing Status = "Transcribing..."
	StatusReady        Status = "Text pasted"
	StatusClipboard    Status = "Text copied"
	StatusError        Status = "Error"
)

func ShowStatus(status Status, detail string) {
	if detail == "" {
		detail = string(status)
	}

	fmt.Printf("%s: %s\n", status, detail)

	// Update the floating GUI state
	switch status {
	case StatusRecording:
		SetGUIState("recording", "")
	case StatusTranscribing:
		SetGUIState("transcribing", "")
	case StatusReady:
		SetGUIState("ready", detail)
	case StatusClipboard:
		SetGUIState("clipboard", detail)
	case StatusError:
		SetGUIState("error", detail)
	}

	if _, err := exec.LookPath("notify-send"); err == nil {
		args := []string{"--app-name", appName}
		switch status {
		case StatusRecording:
			args = append(args, "--urgency", "normal", "--expire-time", "0", string(status), detail)
		case StatusTranscribing:
			args = append(args, "--urgency", "normal", "--expire-time", "0", string(status), detail)
		case StatusError:
			args = append(args, "--urgency", "critical", string(status), detail)
		default:
			args = append(args, "--urgency", "low", "--expire-time", "2500", string(status), detail)
		}

		_ = exec.Command("notify-send", args...).Start()
	}
}
