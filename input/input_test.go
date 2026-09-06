package input_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/DiLRandI/confgen/input"
)

type endless struct{ read int }

func (r *endless) Read(p []byte) (int, error) { clear(p); r.read += len(p); return len(p), nil }

func TestLimitBoundary(t *testing.T) {
	for _, n := range []int{7, 8, 9} {
		data := bytes.Repeat([]byte("x"), n)
		got, err := input.Limit(8).Read(bytes.NewReader(data))
		var size *input.SizeError
		if n > 8 {
			if !errors.As(err, &size) || size.Limit != 8 || got != nil {
				t.Fatalf("oversize: %q %v", got, err)
			}
		} else if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("boundary %d: %q %v", n, got, err)
		}
		if err := input.Limit(8).Check(data); (err != nil) != (n > 8) {
			t.Fatalf("check %d: %v", n, err)
		}
	}
}
func TestReadStopsAtBudget(t *testing.T) {
	r := new(endless)
	data, err := input.Limit(1024).Read(r)
	var size *input.SizeError
	if data != nil || !errors.As(err, &size) || r.read != 1025 {
		t.Fatalf("read %d, error %v", r.read, err)
	}
}
func TestLimits(t *testing.T) {
	if _, err := input.Limit(-1).Read(bytes.NewReader(nil)); err == nil {
		t.Fatal("negative limit accepted")
	}
	if _, err := input.Limit(0).Read(nil); err == nil {
		t.Fatal("nil reader accepted")
	}
	if got, err := input.Limit(math.MaxInt64).Read(bytes.NewBufferString("ok")); err != nil || string(got) != "ok" {
		t.Fatalf("max limit: %q %v", got, err)
	}
	data := make([]byte, int(input.DefaultLimit)+1)
	if err := input.Limit(0).Check(data[:len(data)-1]); err != nil {
		t.Fatal(err)
	}
	var size *input.SizeError
	if err := input.Limit(0).Check(data); !errors.As(err, &size) {
		t.Fatalf("default: %v", err)
	}
}
func TestReadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input")
	if err := os.WriteFile(path, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	if got, err := input.Limit(5).ReadFile(path); err != nil || string(got) != "12345" {
		t.Fatalf("boundary %q %v", got, err)
	}
	if got, err := input.Limit(4).ReadFile(path); got != nil || err == nil {
		t.Fatalf("oversize %q %v", got, err)
	}
}

type broken struct{}

func (broken) Read(p []byte) (int, error) { copy(p, "secret"); return 6, io.ErrUnexpectedEOF }
func TestReadErrorReturnsNoPartialData(t *testing.T) {
	data, err := input.Limit(20).Read(broken{})
	if data != nil || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("partial data: %q %v", data, err)
	}
}
