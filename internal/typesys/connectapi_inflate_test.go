package typesys

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"
)

func TestInflateConnectAPIShape(t *testing.T) {
	for _, input := range []string{"", "header\n", strings.Repeat("type\tvalue\n", 10000)} {
		var archive bytes.Buffer
		writer := gzip.NewWriter(&archive)
		if _, err := writer.Write([]byte(input)); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		got, err := inflateConnectAPIShape(archive.Bytes())
		if err != nil || got != input {
			t.Fatalf("inflate length %d: got length %d, error %v", len(input), len(got), err)
		}
		if _, err := inflateConnectAPIShape(archive.Bytes()[:archive.Len()-1]); err == nil {
			t.Fatal("truncated archive accepted")
		}
	}
	if _, err := inflateConnectAPIShape([]byte("not gzip")); err == nil {
		t.Fatal("invalid archive accepted")
	}
}

func BenchmarkConnectAPIShapeDecode(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := decodeConnectAPIShape(connectAPIShapeArchive); err != nil {
			b.Fatal(err)
		}
	}
}
