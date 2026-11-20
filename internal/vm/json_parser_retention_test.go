package vm

import (
	"runtime"
	"strings"
	"testing"
)

func BenchmarkJSONParserRetainedAfterEOF(b *testing.B) {
	input := "[" + strings.Repeat("0,", 32767) + "0]"
	for i := 0; i < b.N; i++ {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		parser, err := newJSONParser(input)
		if err != nil {
			b.Fatal(err)
		}
		_, parser, _, _, err = jsonParserNextToken(parser)
		if err != nil {
			b.Fatal(err)
		}
		parser, err = jsonParserSkipChildren(parser)
		if err != nil {
			b.Fatal(err)
		}
		_, parser, _, _, err = jsonParserNextToken(parser)
		if err != nil {
			b.Fatal(err)
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		runtime.KeepAlive(parser)
		b.ReportMetric(float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)), "retained-B")
	}
}
