package main

import (
	"dictate/src"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	recorder, err := src.NewRecorder()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	var mu sync.Mutex
	isRecording := false
	isTranscribing := false

	toggle := func() {
		mu.Lock()
		if isTranscribing {
			mu.Unlock()
			src.ShowStatus(src.StatusTranscribing, "Please wait for the current transcription to finish.")
			return
		}

		if !isRecording {
			if err := recorder.Start(); err != nil {
				mu.Unlock()
				src.ShowStatus(src.StatusError, err.Error())
				return
			}
			isRecording = true
			mu.Unlock()
			src.ShowStatus(src.StatusRecording, "Speak now. Press Ctrl+Space again to stop.")
		} else {
			path, err := recorder.Stop()
			if err != nil {
				isRecording = false
				mu.Unlock()
				src.ShowStatus(src.StatusError, err.Error())
				return
			}
			isRecording = false
			isTranscribing = true
			mu.Unlock()
			src.ShowStatus(src.StatusTranscribing, "Converting speech to text...")

			go func() {
				defer os.Remove(path)
				defer func() {
					mu.Lock()
					isTranscribing = false
					mu.Unlock()
				}()

				text, err := src.Transcribe(path)
				if err != nil {
					src.ShowStatus(src.StatusError, err.Error())
					return
				}

				result, err := src.DeliverText(text)
				if err != nil {
					src.ShowStatus(src.StatusError, err.Error())
					return
				}

				if result == src.DeliveryPasted {
					src.ShowStatus(src.StatusReady, "Transcription pasted into the focused input.")
				} else {
					src.ShowStatus(src.StatusClipboard, "Paste with Ctrl+V when ready.")
				}
			}()
		}
	}

	if err := setupHotkey(toggle); err != nil {
		fmt.Println("[ERROR] Failed to setup hotkey:", err)
		os.Exit(1)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\nShutting down...")
	recorder.Terminate()
	os.Remove(pipePath)
}
