package main

import (
	"dictate/src"
	"fmt"
	"os"
	"time"
)

func main() {
	recorder, err := src.NewRecorder()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer recorder.Terminate()

	fmt.Println("Recording for 5 seconds...")
	if err := recorder.Start(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	time.Sleep(5 * time.Second)

	fmt.Println("Stopped. Transcribing...")
	path, err := recorder.Stop()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer os.Remove(path)

	recorder.Terminate()

	text, err := src.Transcribe(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Transcription:", text)
}
