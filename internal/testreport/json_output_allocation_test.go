package testreport

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"testing"
)

func largeJSONReport() Run {
	run := Run{Name: "synthetic"}
	for i := 0; i < 100; i++ {
		suite := Suite{Name: fmt.Sprintf("Suite%d", i)}
		for j := 0; j < 10; j++ {
			suite.Cases = append(suite.Cases, Case{MethodName: fmt.Sprintf("method%d", j), Status: StatusPass, DurationMS: int64(j)})
		}
		run.Suites = append(run.Suites, suite)
	}
	return run
}

func TestReportJSONEncoderCompatibility(t *testing.T) {
	for name, run := range map[string]Run{"large": largeJSONReport(), "empty": {}, "failure": {Suites: []Suite{{Name: "<&>", Cases: []Case{{Status: StatusFail, Problem: &Problem{Message: "bad <&> \u2028"}}}}}}} {
		t.Run(name, func(t *testing.T) {
			value := NativePerTestRows(run)
			var want, got bytes.Buffer
			enc := json.NewEncoder(&want)
			enc.SetIndent("", "  ")
			if err := enc.Encode(value); err != nil {
				t.Fatal(err)
			}
			if err := writeReportJSON(&got, value); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got.Bytes(), want.Bytes()) {
				t.Fatal("encoded bytes differ")
			}
			t.Logf("json sha256 %x", sha256.Sum256(got.Bytes()))
		})
	}
}

func BenchmarkLargeReportJSON(b *testing.B) {
	value := NativePerTestRows(largeJSONReport())
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := writeReportJSON(io.Discard, value); err != nil {
			b.Fatal(err)
		}
	}
}

type reportJSONErrorWriter struct{ writes int }

func (w *reportJSONErrorWriter) Write([]byte) (int, error) { w.writes++; return 0, io.ErrClosedPipe }

func TestReportJSONWriterError(t *testing.T) {
	w := &reportJSONErrorWriter{}
	err := writeReportJSON(w, largeJSONReport())
	if !errors.Is(err, io.ErrClosedPipe) || w.writes != 1 {
		t.Fatalf("error = %v, writes = %d", err, w.writes)
	}
}
