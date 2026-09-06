package input_test

import (
	"bytes"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/DiLRandI/confgen/input"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type endless struct{ read int }

func (r *endless) Read(p []byte) (int, error) { clear(p); r.read += len(p); return len(p), nil }

func TestLimitBoundary(t *testing.T) {
	t.Parallel()
	for _, n := range []int{7, 8, 9} {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			t.Parallel()
			data := bytes.Repeat([]byte("x"), n)
			got, err := input.Limit(8).Read(bytes.NewReader(data))
			if n > 8 {
				var size *input.SizeError
				require.ErrorAs(t, err, &size)
				assert.Equal(t, input.Limit(8), size.Limit)
				assert.Nil(t, got)
				require.Error(t, input.Limit(8).Check(data))
			} else {
				require.NoError(t, err)
				assert.Equal(t, data, got)
				require.NoError(t, input.Limit(8).Check(data))
			}
		})
	}
}

func TestReadStopsAtBudget(t *testing.T) {
	t.Parallel()
	r := new(endless)
	data, err := input.Limit(1024).Read(r)
	var size *input.SizeError
	require.ErrorAs(t, err, &size)
	assert.Nil(t, data)
	assert.Equal(t, 1025, r.read)
}

func TestLimits(t *testing.T) {
	_, err := input.Limit(-1).Read(bytes.NewReader(nil))
	require.Error(t, err, "negative limit accepted")
	_, err = input.Limit(0).Read(nil)
	require.Error(t, err, "nil reader accepted")
	got, err := input.Limit(math.MaxInt64).Read(bytes.NewBufferString("ok"))
	require.NoError(t, err)
	assert.Equal(t, "ok", string(got))
	data := make([]byte, int(input.DefaultLimit)+1)
	require.NoError(t, input.Limit(0).Check(data[:len(data)-1]))
	var size *input.SizeError
	require.ErrorAs(t, input.Limit(0).Check(data), &size)
}

func TestReadFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "input")
	require.NoError(t, os.WriteFile(path, []byte("12345"), 0o600))
	got, err := input.Limit(5).ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, "12345", string(got))
	got, err = input.Limit(4).ReadFile(path)
	require.Error(t, err)
	assert.Nil(t, got)
}

type broken struct{}

func (broken) Read(p []byte) (int, error) { copy(p, "secret"); return 6, io.ErrUnexpectedEOF }
func TestReadErrorReturnsNoPartialData(t *testing.T) {
	t.Parallel()
	data, err := input.Limit(20).Read(broken{})
	require.ErrorIs(t, err, io.ErrUnexpectedEOF)
	assert.Nil(t, data)
}
