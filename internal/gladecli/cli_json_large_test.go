package gladecli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"testing"
)

func largeCLIEnvelopes() map[string]cliJSONEnvelope {
	rows := make([]map[string]any, 500)
	for i := range rows {
		rows[i] = map[string]any{"name": fmt.Sprintf("Class%d", i), "status": "passed", "diagnostics": []string{}, "message": "quoted <&> text\u2028"}
	}
	return map[string]cliJSONEnvelope{
		"check":  {Command: "check", Status: "passed", Summary: map[string]any{"types": 500}, Diagnostics: []any{}, Data: rows},
		"test":   {Command: "test", Status: "passed", Summary: map[string]any{"total": 500, "passed": 500}, Tests: rows},
		"report": {Command: "report assess", Status: "passed", Data: map[string]any{"findings": rows}},
	}
}

func TestLargeCLIJSONEnvelopeCompatibility(t *testing.T) {
	for name, env := range largeCLIEnvelopes() {
		t.Run(name, func(t *testing.T) {
			env.SchemaVersion = cliJSONSchemaVersion
			var want, got bytes.Buffer
			enc := json.NewEncoder(&want)
			enc.SetIndent("", "  ")
			if err := enc.Encode(env); err != nil {
				t.Fatal(err)
			}
			if err := writeLargeCLIJSONEnvelope(&got, env); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got.Bytes(), want.Bytes()) {
				t.Fatal("encoded bytes differ")
			}
			t.Logf("json sha256 %x", sha256.Sum256(got.Bytes()))
		})
	}
}

func BenchmarkWriteLargeCLIJSONEnvelope(b *testing.B) {
	for name, env := range largeCLIEnvelopes() {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := writeLargeCLIJSONEnvelope(io.Discard, env); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
