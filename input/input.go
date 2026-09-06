// Package input bounds configuration and schema input before parsing.
package input

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// DefaultLimit allows 32 MiB per document. A zero Limit selects this default.
const DefaultLimit Limit = 32 << 20

// Limit is a byte budget. Positive values override the default; negatives are invalid.
type Limit int64

// SizeError reports an input that exceeds its byte budget without retaining data.
type SizeError struct{ Limit Limit }

func (e *SizeError) Error() string { return fmt.Sprintf("input exceeds %d byte limit", e.Limit) }

func (l Limit) budget() (Limit, error) {
	if l < 0 {
		return 0, errors.New("input byte limit must not be negative")
	}
	if l == 0 {
		return DefaultLimit, nil
	}
	return l, nil
}

// Check validates the size of an already allocated document.
func (l Limit) Check(data []byte) error {
	limit, err := l.budget()
	if err != nil {
		return err
	}
	if int64(len(data)) > int64(limit) {
		return &SizeError{Limit: limit}
	}
	return nil
}

// Read consumes at most the budget plus one byte and returns no data on failure.
func (l Limit) Read(r io.Reader) ([]byte, error) {
	limit, err := l.budget()
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, errors.New("nil input reader")
	}
	// Read the extra byte separately so the largest int64 budget cannot overflow.
	data, err := io.ReadAll(io.LimitReader(r, int64(limit)))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) < int64(limit) {
		return data, nil
	}
	var extra [1]byte
	n, err := io.ReadFull(r, extra[:])
	if n != 0 {
		return nil, &SizeError{Limit: limit}
	}
	if err != nil && err != io.EOF {
		return nil, err
	}
	return data, nil
}

// ReadFile bounds reads even when a file grows or reports an unreliable size.
func (l Limit) ReadFile(path string) ([]byte, error) {
	if _, err := l.budget(); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return l.Read(f)
}
