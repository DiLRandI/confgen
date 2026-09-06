package document

import (
	"bytes"
	"strconv"
	"testing"
)

func TestPositionsMatchReference(t *testing.T) {
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
		if line != expectedLine || column != expectedColumn {
			t.Fatalf("offset %d: got %d:%d, want %d:%d", offset, line, column, expectedLine, expectedColumn)
		}
	}
}

func TestJSONDuplicateKeyOffset(t *testing.T) {
	data := []byte("{\n  \"na\\\"me\": 1,\n  \"na\\\"me\": 2\n}")
	_, err := Parse(data, true)
	e, ok := err.(*Error)
	if !ok || e.Kind != "duplicate" {
		t.Fatalf("got %v, want duplicate error", err)
	}
	if e.Line != 3 || e.Column != 3 {
		t.Fatalf("got %d:%d, want 3:3", e.Line, e.Column)
	}
}

func TestJSONNodePositions(t *testing.T) {
	data := []byte("{\n  \"nested\": {\n    \"items\": [\n      {\"value\": 1}\n    ]\n  }\n}")
	root, err := Parse(data, true)
	if err != nil {
		t.Fatal(err)
	}
	if root.Line != 1 || root.Column != 1 {
		t.Fatalf("root at %d:%d, want 1:1", root.Line, root.Column)
	}
	nested := root.Fields["nested"]
	if nested.Line != 2 || nested.Column != 11 {
		t.Fatalf("nested at %d:%d, want 2:11", nested.Line, nested.Column)
	}
	items := nested.Fields["items"]
	if items.Line != 3 || items.Column != 12 {
		t.Fatalf("items at %d:%d, want 3:12", items.Line, items.Column)
	}
	item := items.Items[0]
	if item.Line != 4 || item.Column != 7 {
		t.Fatalf("item at %d:%d, want 4:7", item.Line, item.Column)
	}
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
