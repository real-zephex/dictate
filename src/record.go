package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/gordonklaus/portaudio"
	pa "github.com/gordonklaus/portaudio"
)

const (
	sampleRate      = 16000
	channels        = 1
	framesPerBuffer = 1024
)

type Recorder struct {
	stream    *pa.Stream
	buffer    []int16
	recording bool
}

func NewRecorder() (*Recorder, error) {
	err := pa.Initialize()
	if err != nil {
		return nil, fmt.Errorf("[ERROR] Failed to initialize PortAudio: %v", err)
	}
	return &Recorder{}, nil
}

func (r *Recorder) Start() error {
	r.buffer = []int16{}
	r.recording = true

	chunk := make([]int16, framesPerBuffer)

	stream, err := pa.OpenDefaultStream(
		channels, // input channels
		0,        // output channels
		float64(sampleRate),
		framesPerBuffer,
		chunk,
	)
	if err != nil {
		return fmt.Errorf("[ERROR] Failed to open audio stream: %v", err)
	}
	r.stream = stream

	if err := stream.Start(); err != nil {
		return fmt.Errorf("[ERROR] Failed to start audio stream: %v", err)
	}

	go func() {
		for r.recording {
			if err := r.stream.Read(); err != nil {
				break
			}
			// append chunk to buffer
			tmp := make([]int16, len(chunk))
			copy(tmp, chunk)
			r.buffer = append(r.buffer, tmp...)
		}
	}()

	return nil
}

func (r *Recorder) Stop() (string, error) {
	r.recording = false
	time.Sleep(50 * time.Millisecond) // let the read goroutine exit

	if r.stream != nil {
		r.stream.Stop()
		r.stream.Close()
		r.stream = nil // nil it out so Terminate doesn't double-free
	}

	return saveWAV(r.buffer)
}

func (r *Recorder) Terminate() {
	// guard against the known Pa_Terminate segfault
	defer func() { recover() }()
	portaudio.Terminate()
}

// saveWAV writes PCM int16 samples to a temp WAV file and returns the path
func saveWAV(samples []int16) (string, error) {
	tmpFile, err := os.CreateTemp("", "whispr-*.wav")
	if err != nil {
		return "", fmt.Errorf("[ERROR] Failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	if err := writeWAVHeader(tmpFile, len(samples)); err != nil {
		return "", err
	}

	if err := binary.Write(tmpFile, binary.LittleEndian, samples); err != nil {
		return "", fmt.Errorf("[ERROR] Failed to write PCM data: %v", err)
	}

	return tmpFile.Name(), nil
}

func writeWAVHeader(f *os.File, numSamples int) error {
	const bitsPerSample = 16
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	dataSize := numSamples * 2 // int16 = 2 bytes
	chunkSize := 36 + dataSize

	buf := new(bytes.Buffer)

	// RIFF header
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(chunkSize))
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16)) // chunk size
	binary.Write(buf, binary.LittleEndian, uint16(1))  // PCM format
	binary.Write(buf, binary.LittleEndian, uint16(channels))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))

	// data chunk
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(dataSize))

	_, err := f.Write(buf.Bytes())
	return err
}
