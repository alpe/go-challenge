package drum

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
)

const (
	spliceTypePattern = "SPLICE"
	typeHeaderLength  = uint8(len(spliceTypePattern))
)

// ErrUnsupportedFileFormat is returned when the file to decode does not match
// the expected format.
var ErrUnsupportedFileFormat = errors.New("unsupported file format")

// DecodeFile decodes the drum machine file found at the provided path
// and returns a pointer to a parsed pattern which is the entry point to the
// rest of the data.
func DecodeFile(path string) (*Pattern, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return decode(bufio.NewReader(file))
}

func decode(r io.Reader) (*Pattern, error) {
	p, err := newPayloadReader(r)
	if err != nil {
		return nil, err
	}
	return decodePattern(p)
}

func newPayloadReader(r io.Reader) (*io.LimitedReader, error) {
	typeHeader, err := readBytes(r, typeHeaderLength)
	if err != nil {
		return nil, fmt.Errorf("parse type header: %v", err)
	}
	if !bytes.Equal(typeHeader, []byte(spliceTypePattern)) {
		return nil, ErrUnsupportedFileFormat
	}
	var payloadSize int64
	if err := binary.Read(r, binary.BigEndian, &payloadSize); err != nil {
		return nil, fmt.Errorf("parse payload size: %v", err)

	}

	return &io.LimitedReader{r, payloadSize}, nil
}

func decodePattern(r *io.LimitedReader) (*Pattern, error) {
	var pattern Pattern
	v, err := readBytes(r, maxVersionLength)
	if err != nil {
		return nil, fmt.Errorf("parse version: %v", err)
	}
	pattern.version = cropToString(v)

	if err := binary.Read(r, binary.LittleEndian, &pattern.tempo); err != nil {
		return nil, fmt.Errorf("parse tempo: %v", err)
	}
	for r.N > 0 {
		tr, err := decodeTrack(r)
		if err != nil {
			return nil, err
		}
		pattern.tracks = append(pattern.tracks, tr)
	}
	return &pattern, nil
}

func decodeTrack(r io.Reader) (*Track, error) {
	var track Track
	if err := binary.Read(r, binary.LittleEndian, &track.id); err != nil {
		return nil, fmt.Errorf("parse track id: %v", err)
	}
	var lenName uint8
	if err := binary.Read(r, binary.LittleEndian, &lenName); err != nil {
		return nil, fmt.Errorf("parse track name length: %v", err)
	}
	b, err := readBytes(r, lenName)
	if err != nil {
		return nil, fmt.Errorf("parse track name: %v", err)
	}
	track.name = string(b)

	if track.steps, err = decodeSteps(r); err != nil {
		return nil, err
	}
	return &track, nil
}

func decodeSteps(r io.Reader) (Steps, error) {
	var steps Steps
	stepsAsBytes, err := readBytes(r, stepsLength)
	if err != nil {
		return steps, fmt.Errorf("parse steps: %v", err)
	}
	for i, v := range stepsAsBytes {
		steps[i] = (v == 1)
	}
	return steps, nil
}

// readBytes reads exactly n bytes from r into a new slice
// The error is EOF only if no bytes were read.
// If an EOF happens after reading some but not all the bytes,
// ReadFull returns ErrUnexpectedEOF.
func readBytes(r io.Reader, n uint8) ([]byte, error) {
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

const endOfString = 0x00

func cropToString(b []byte) string {
	n := bytes.Index(b, []byte{endOfString})
	if n < 0 {
		n = len(b)
	}
	return string(b[:n])
}
