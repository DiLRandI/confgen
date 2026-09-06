package document

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPositionsMatchReference(t *testing.T) {
	t.Parallel()
	data := []byte("first\nsecond\n\nlast")
	p := newPositions(data)
	for offset := 0; offset <= len(data)+2; offset++ {
		line, column := p.position(offset)
		expectedLine, expectedColumn := 1, 1
		for i, b := range data {
			if i >= offset {
				break
			}
			if b == '\n' {
				expectedLine++
				expectedColumn = 1
			} else {
				expectedColumn++
			}
		}
		require.Equal(t, expectedLine, line, "offset %d: got %d:%d, want %d:%d", offset, line, column, expectedLine, expectedColumn)
		require.Equal(t, expectedColumn, column, "offset %d: got %d:%d, want %d:%d", offset, line, column, expectedLine, expectedColumn)
	}
}

func TestJSONDuplicateKeyOffset(t *testing.T) {
	t.Parallel()
	data := []byte("{\n  \"na\\\"me\": 1,\n  \"na\\\"me\": 2\n}")
	_, err := Parse(data, true)
	e, ok := err.(*Error)
	require.True(t, ok, "got %v, want duplicate error", err)
	require.Equal(t, "duplicate", e.Kind, "got %v, want duplicate error", err)
	require.Equal(t, 3, e.Line, "got %d:%d, want 3:3", e.Line, e.Column)
	require.Equal(t, 3, e.Column, "got %d:%d, want 3:3", e.Line, e.Column)
}

func TestJSONNodePositions(t *testing.T) {
	t.Parallel()
	data := []byte("{\n  \"nested\": {\n    \"items\": [\n      {\"value\": 1}\n    ]\n  }\n}")
	root, err := Parse(data, true)
	require.NoError(t, err)
	require.Equal(t, 1, root.Line, "root at %d:%d, want 1:1", root.Line, root.Column)
	require.Equal(t, 1, root.Column, "root at %d:%d, want 1:1", root.Line, root.Column)
	nested := root.Fields["nested"]
	require.Equal(t, 2, nested.Line, "nested at %d:%d, want 2:11", nested.Line, nested.Column)
	require.Equal(t, 11, nested.Column, "nested at %d:%d, want 2:11", nested.Line, nested.Column)
	items := nested.Fields["items"]
	require.Equal(t, 3, items.Line, "items at %d:%d, want 3:12", items.Line, items.Column)
	require.Equal(t, 12, items.Column, "items at %d:%d, want 3:12", items.Line, items.Column)
	item := items.Items[0]
	require.Equal(t, 4, item.Line, "item at %d:%d, want 4:7", item.Line, item.Column)
	require.Equal(t, 7, item.Column, "item at %d:%d, want 4:7", item.Line, item.Column)
}

func BenchmarkJSONLargeDocument(b *testing.B) {
	var data bytes.Buffer
	data.WriteByte('{')
	for i := 0; i < 10000; i++ {
		if i > 0 {
			data.WriteByte(',')
		}
		data.WriteString("\n\"field")
		data.WriteString(strconv.Itoa(i))
		data.WriteString("\": 1")
	}
	data.WriteString("\n}")
	input := data.Bytes()
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Parse(input, true); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkJSONPositionLookupLargeDocument(b *testing.B) {
	for _, lines := range []int{1_000, 10_000, 100_000} {
		b.Run(strconv.Itoa(lines), func(b *testing.B) {
			data := bytes.Repeat([]byte("value\n"), lines)
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				p := newPositions(data)
				for offset := 0; offset < len(data); offset += len("value\n") {
					p.position(offset)
				}
			}
		})
	}
}
