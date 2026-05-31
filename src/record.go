package src

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sync"

	"github.com/gordonklaus/portaudio"
	pa "github.com/gordonklaus/portaudio"
)

const (
	sampleRate      = 16000
	channels        = 1
	framesPerBuffer = 1024
)

type Recorder struct {
	mu        sync.Mutex
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
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.recording {
		return fmt.Errorf("[ERROR] Recorder is already running")
	}

	r.buffer = []int16{}
	r.recording = true

	stream, err := pa.OpenDefaultStream(
		channels, // input channels
		0,        // output channels
		float64(sampleRate),
		framesPerBuffer,
		r.processAudio,
	)
	if err != nil {
		r.recording = false
		return fmt.Errorf("[ERROR] Failed to open audio stream: %v", err)
	}
	r.stream = stream

	if err := stream.Start(); err != nil {
		stream.Close()
		r.stream = nil
		r.recording = false
		return fmt.Errorf("[ERROR] Failed to start audio stream: %v", err)
	}

	return nil
}

func (r *Recorder) processAudio(in []int16) {
	tmp := make([]int16, len(in))
	copy(tmp, in)

	r.mu.Lock()
	if r.recording {
		r.buffer = append(r.buffer, tmp...)
	}
	r.mu.Unlock()
}

func (r *Recorder) Stop() (string, error) {
	r.mu.Lock()
	if !r.recording {
		r.mu.Unlock()
		return "", fmt.Errorf("[ERROR] Recorder is not running")
	}

	r.recording = false
	stream := r.stream
	r.mu.Unlock()

	if stream != nil {
		if err := stream.Stop(); err != nil {
			_ = stream.Abort()
		}
		if err := stream.Close(); err != nil {
			return "", fmt.Errorf("[ERROR] Failed to close audio stream: %v", err)
		}
	}

	r.mu.Lock()
	r.stream = nil
	samples := append([]int16(nil), r.buffer...)
	r.mu.Unlock()

	return saveWAV(samples)
}

func (r *Recorder) Terminate() {
	r.mu.Lock()
	recording := r.recording
	r.mu.Unlock()
	if recording {
		_, _ = r.Stop()
	}

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
