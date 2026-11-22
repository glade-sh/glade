package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"testing"
)

func serverJSONShape() any {
	rows := make([]map[string]any, 1000)
	for i := range rows {
		rows[i] = map[string]any{"Id": fmt.Sprintf("row-%d", i), "Name": "<record> & value", "empty": []string{}, "nullable": nil}
	}
	return map[string]any{"totalSize": len(rows), "done": true, "records": rows}
}

func TestServerJSONEncoderCompatibility(t *testing.T) {
	for name, value := range map[string]any{"query": serverJSONShape(), "neighbor": map[string]any{"text": "\u2028\xff", "raw": json.RawMessage(`{"z":1}`)}, "error": math.NaN()} {
		t.Run(name, func(t *testing.T) {
			var want, got bytes.Buffer
			wantErr := json.NewEncoder(&want).Encode(value)
			gotErr := writeServerJSON(&got, value)
			if (gotErr == nil) != (wantErr == nil) || gotErr != nil && gotErr.Error() != wantErr.Error() {
				t.Fatalf("errors = %v, want %v", gotErr, wantErr)
			}
			if !bytes.Equal(got.Bytes(), want.Bytes()) {
				t.Fatal("encoded bytes differ")
			}
		})
	}
}

func BenchmarkServerQueryJSON(b *testing.B) {
	value := serverJSONShape()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := writeServerJSON(io.Discard, value); err != nil {
			b.Fatal(err)
		}
	}
}
