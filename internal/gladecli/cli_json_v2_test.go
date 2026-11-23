//go:build goexperiment.jsonv2

package gladecli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

func parseJSONBenchmarkEnvelope() cliJSONEnvelope {
	result := apexast.Result{Files: make([]apexast.File, 0, 1000)}
	for i := 0; i < 1000; i++ {
		decl := apexast.Declaration{Kind: apexast.DeclarationClass, Name: fmt.Sprintf("Example%d", i)}
		for j := 0; j < 20; j++ {
			decl.Members = append(decl.Members, apexast.Declaration{Kind: apexast.DeclarationMethod, Name: fmt.Sprintf("method%d", j), Type: "String"})
		}
		result.Files = append(result.Files, apexast.File{Path: fmt.Sprintf("src/Example%d.cls", i), Kind: apexast.FileKindClass, Declarations: []apexast.Declaration{decl}})
	}
	return cliJSONEnvelope{SchemaVersion: "1.0", Command: "parse", Status: "passed", Data: result}
}

func TestMarshalCLIJSONEnvelopeCompatibility(t *testing.T) {
	for name, env := range map[string]cliJSONEnvelope{
		"parse":  parseJSONBenchmarkEnvelope(),
		"empty":  {SchemaVersion: "1.0", Command: "version", Status: "passed"},
		"values": {SchemaVersion: "1.0", Data: map[string]any{"html": "<&>\u2028", "invalid": string([]byte{0xff}), "raw": json.RawMessage(`{"b":2,"a":1}`), "nil": []string(nil), "empty": []string{}}},
		"error":  {Data: math.NaN()},
	} {
		t.Run(name, func(t *testing.T) {
			var want bytes.Buffer
			encoder := json.NewEncoder(&want)
			encoder.SetIndent("", "  ")
			wantErr := encoder.Encode(env)
			got, gotErr := marshalCLIJSONEnvelope(env)
			if (gotErr == nil) != (wantErr == nil) || (gotErr != nil && gotErr.Error() != wantErr.Error()) {
				t.Fatalf("errors: got %v, want %v", gotErr, wantErr)
			}
			if !bytes.Equal(got, want.Bytes()) {
				t.Fatalf("output differs: got %x, want %x", sha256.Sum256(got), sha256.Sum256(want.Bytes()))
			}
			if name == "parse" {
				t.Logf("parse output sha256 %x", sha256.Sum256(got))
			}
		})
	}
}

type parseJSONErrorWriter struct {
	writes int
}

func (w *parseJSONErrorWriter) Write(p []byte) (int, error) {
	w.writes++
	return 0, io.ErrClosedPipe
}

func TestWriteParseCLIJSONEnvelopeWriterError(t *testing.T) {
	w := &parseJSONErrorWriter{}
	err := writeParseCLIJSONEnvelope(w, cliJSONEnvelope{Command: "parse"})
	if !errors.Is(err, io.ErrClosedPipe) || w.writes != 1 {
		t.Fatalf("error = %v, writes = %d", err, w.writes)
	}
	w = &parseJSONErrorWriter{}
	err = writeParseCLIJSONEnvelope(w, cliJSONEnvelope{Data: math.NaN()})
	if err == nil || w.writes != 0 {
		t.Fatalf("marshal error = %v, writes = %d", err, w.writes)
	}
}

func BenchmarkWriteCLIJSONEnvelopeParse(b *testing.B) {
	env := parseJSONBenchmarkEnvelope()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := writeParseCLIJSONEnvelope(io.Discard, env); err != nil {
			b.Fatal(err)
		}
	}
}
