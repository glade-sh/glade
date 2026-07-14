package testdaemon

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/watch"
)

func TestProtocolV1RequestRoundTripPreservesAllRunPolicy(t *testing.T) {
	want := representativeProtocolV1Request()

	var encoded bytes.Buffer
	if err := EncodeRequestV1(&encoded, want); err != nil {
		t.Fatalf("EncodeRequestV1: %v", err)
	}
	got, err := DecodeRequestV1(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatalf("DecodeRequestV1: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("request round trip mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestProtocolV1RequestEncodingPreservesExplicitZeroAndFalse(t *testing.T) {
	want := RequestV1{
		Version: ProtocolVersionV1,
		Op:      OpRun,
		Run:     &RunRequestV1{},
	}

	var encoded bytes.Buffer
	if err := EncodeRequestV1(&encoded, want); err != nil {
		t.Fatalf("EncodeRequestV1: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(bytes.TrimSuffix(encoded.Bytes(), []byte{'\n'}), &envelope); err != nil {
		t.Fatalf("decode request JSON as map: %v", err)
	}
	run := requireProtocolV1Map(t, envelope, "run")
	for key, zero := range map[string]any{
		"filter":              "",
		"changedSince":        "",
		"selectedMethod":      "",
		"limitMode":           "",
		"limitCapsSet":        false,
		"traceBlocked":        false,
		"traceAll":            false,
		"slowTestThresholdMs": float64(0),
		"timeoutMs":           float64(0),
		"parallelism":         float64(0),
		"parallelMethods":     false,
		"noDiskCache":         false,
		"perfCounters":        false,
		"shardCount":          float64(0),
		"shardIndex":          float64(0),
		"returnClassShards":   false,
	} {
		requireProtocolV1JSONValue(t, run, key, zero)
	}
	for _, key := range []string{"selectedClasses", "classDurationMs", "methodDurationMs"} {
		if _, ok := run[key]; !ok {
			t.Errorf("request run policy omitted %q", key)
		}
	}
	caps := requireProtocolV1Map(t, run, "limitCaps")
	for _, key := range protocolV1LimitCapJSONKeys() {
		requireProtocolV1JSONValue(t, caps, key, float64(0))
	}

	got, err := DecodeRequestV1(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatalf("DecodeRequestV1: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("zero-valued request round trip mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestProtocolV1ResponseRoundTripPreservesRunSelectionAndShardPlan(t *testing.T) {
	want := ResponseV1{
		Version: ProtocolVersionV1,
		Op:      OpRunResult,
		ID:      "request-17",
		OK:      false,
		Error:   "expected failure",
		Ready:   false,
		Warming: false,
		Project: "/workspace/example",
		Run: &testreport.Run{
			Name: "representative",
			Dependencies: []typesys.DependencyInfo{{
				Namespace:  "example",
				SourceRoot: "dependencies/example",
				Version:    "1.2.3",
				Status:     "loaded",
				ApexTypes:  3,
			}},
			Suites: []testreport.Suite{{
				Name: "InvoiceTest",
				Cases: []testreport.Case{{
					ClassName:  "InvoiceTest",
					MethodName: "calculatesTotal",
					Status:     testreport.StatusPass,
				}},
			}},
		},
		Selection: &watch.TestSelection{
			Mode:        watch.SelectionDirect,
			TestClasses: []string{"InvoiceTest"},
			Reason:      "direct dependency",
		},
		ShardPlan: &ClassShardPlanV1{Shards: []ClassShardV1{
			{},
			{Index: 1, TotalDurationMS: 321, Classes: []string{"InvoiceTest", "TaxTest"}},
		}},
	}

	var encoded bytes.Buffer
	if err := EncodeResponseV1(&encoded, want); err != nil {
		t.Fatalf("EncodeResponseV1: %v", err)
	}
	var envelope map[string]any
	if err := json.Unmarshal(bytes.TrimSuffix(encoded.Bytes(), []byte{'\n'}), &envelope); err != nil {
		t.Fatalf("decode response JSON as map: %v", err)
	}
	for _, key := range []string{"ok", "ready", "warming"} {
		requireProtocolV1JSONValue(t, envelope, key, false)
	}
	shardPlan := requireProtocolV1Map(t, envelope, "shardPlan")
	shards, ok := shardPlan["shards"].([]any)
	if !ok || len(shards) != 2 {
		t.Fatalf("shardPlan.shards = %#v, want two shards", shardPlan["shards"])
	}
	zeroShard, ok := shards[0].(map[string]any)
	if !ok {
		t.Fatalf("first shard = %#v, want object", shards[0])
	}
	requireProtocolV1JSONValue(t, zeroShard, "index", float64(0))
	requireProtocolV1JSONValue(t, zeroShard, "totalDurationMs", float64(0))
	if _, ok := zeroShard["classes"]; !ok {
		t.Error("zero-valued shard omitted classes")
	}

	got, err := DecodeResponseV1(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatalf("DecodeResponseV1: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("response round trip mismatch:\n got: %#v\nwant: %#v", got, want)
	}
}

func TestProtocolV1DecodeRejectsUnknownFields(t *testing.T) {
	tests := []struct {
		name   string
		frame  string
		decode func(io.Reader) error
	}{
		{
			name:  "request envelope",
			frame: `{"version":1,"op":"run","unknownEnvelope":true}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeRequestV1(r)
				return err
			},
		},
		{
			name:  "request run policy",
			frame: `{"version":1,"op":"run","run":{"unknownRunPolicy":true}}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeRequestV1(r)
				return err
			},
		},
		{
			name:  "request limit caps",
			frame: `{"version":1,"op":"run","run":{"limitCaps":{"unknownLimit":1}}}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeRequestV1(r)
				return err
			},
		},
		{
			name:  "response envelope",
			frame: `{"version":1,"op":"run_result","ok":false,"ready":false,"warming":false,"unknownEnvelope":true}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeResponseV1(r)
				return err
			},
		},
		{
			name:  "response run",
			frame: `{"version":1,"op":"run_result","ok":false,"ready":false,"warming":false,"run":{"summary":{},"suites":[],"unknownRun":true}}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeResponseV1(r)
				return err
			},
		},
		{
			name:  "response run summary",
			frame: `{"version":1,"op":"run_result","ok":false,"ready":false,"warming":false,"run":{"summary":{"unknownSummary":true},"suites":[]}}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeResponseV1(r)
				return err
			},
		},
		{
			name:  "response run suite",
			frame: `{"version":1,"op":"run_result","ok":false,"ready":false,"warming":false,"run":{"summary":{},"suites":[{"name":"InvoiceTest","cases":[],"unknownSuite":true}]}}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeResponseV1(r)
				return err
			},
		},
		{
			name:  "response run case",
			frame: `{"version":1,"op":"run_result","ok":false,"ready":false,"warming":false,"run":{"summary":{"total":1,"passed":1},"suites":[{"name":"InvoiceTest","cases":[{"status":"pass","unknownCase":true}]}]}}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeResponseV1(r)
				return err
			},
		},
		{
			name:  "response shard plan",
			frame: `{"version":1,"op":"run_result","ok":false,"ready":false,"warming":false,"shardPlan":{"shards":[],"unknownPlan":true}}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeResponseV1(r)
				return err
			},
		},
		{
			name:  "response class shard",
			frame: `{"version":1,"op":"run_result","ok":false,"ready":false,"warming":false,"shardPlan":{"shards":[{"index":0,"totalDurationMs":0,"classes":[],"unknownShard":true}]}}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeResponseV1(r)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.decode(strings.NewReader(tt.frame))
			if err == nil {
				t.Fatal("decode succeeded, want unknown-field error")
			}
			if !strings.Contains(strings.ToLower(err.Error()), "unknown") {
				t.Fatalf("decode error = %q, want unknown-field identification", err)
			}
		})
	}
}

func TestProtocolV1ResponseRejectsInconsistentRunSummary(t *testing.T) {
	frame := `{"version":1,"op":"run_result","ok":true,"ready":true,"warming":false,"run":{"summary":{"total":2,"passed":2},"suites":[{"name":"InvoiceTest","cases":[{"status":"pass"}]}]}}` + "\n"
	_, err := DecodeResponseV1(strings.NewReader(frame))
	if err == nil {
		t.Fatal("DecodeResponseV1 accepted a run summary that disagrees with the decoded suites")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "summary") {
		t.Fatalf("DecodeResponseV1 error = %q, want summary mismatch", err)
	}
}

func TestProtocolV1DecodeRejectsTrailingJSON(t *testing.T) {
	requestJSON := `{"version":1,"op":"ping"}`
	responseJSON := `{"version":1,"op":"pong","ok":true,"ready":true,"warming":false}`
	tests := []struct {
		name   string
		frame  string
		decode func(io.Reader) error
		valid  bool
	}{
		{
			name:  "request second object",
			frame: requestJSON + `{}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeRequestV1(r)
				return err
			},
		},
		{
			name:  "request trailing token",
			frame: requestJSON + ` x` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeRequestV1(r)
				return err
			},
		},
		{
			name:  "request trailing whitespace",
			frame: requestJSON + " \t\r\n",
			decode: func(r io.Reader) error {
				_, err := DecodeRequestV1(r)
				return err
			},
			valid: true,
		},
		{
			name:  "response second object",
			frame: responseJSON + `{}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeResponseV1(r)
				return err
			},
		},
		{
			name:  "response trailing token",
			frame: responseJSON + ` x` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeResponseV1(r)
				return err
			},
		},
		{
			name:  "response trailing whitespace",
			frame: responseJSON + " \t\r\n",
			decode: func(r io.Reader) error {
				_, err := DecodeResponseV1(r)
				return err
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.decode(strings.NewReader(tt.frame))
			if tt.valid {
				if err != nil {
					t.Fatalf("decode rejected whitespace-only trailer: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("decode accepted trailing JSON")
			}
		})
	}
}

func TestProtocolV1RejectsInvalidVersion(t *testing.T) {
	versions := []struct {
		name    string
		version int
		json    string
	}{
		{name: "missing", version: 0, json: ""},
		{name: "zero", version: 0, json: `"version":0,`},
		{name: "negative", version: -1, json: `"version":-1,`},
		{name: "unsupported", version: 2, json: `"version":2,`},
	}

	for _, tt := range versions {
		t.Run("encode request "+tt.name, func(t *testing.T) {
			var dst bytes.Buffer
			err := EncodeRequestV1(&dst, RequestV1{Version: tt.version, Op: OpPing})
			requireProtocolV1VersionError(t, err, tt.version)
			if dst.Len() != 0 {
				t.Fatalf("invalid request encode wrote %d bytes", dst.Len())
			}
		})
		t.Run("encode response "+tt.name, func(t *testing.T) {
			var dst bytes.Buffer
			err := EncodeResponseV1(&dst, ResponseV1{Version: tt.version, Op: OpPong})
			requireProtocolV1VersionError(t, err, tt.version)
			if dst.Len() != 0 {
				t.Fatalf("invalid response encode wrote %d bytes", dst.Len())
			}
		})
		t.Run("decode request "+tt.name, func(t *testing.T) {
			frame := "{" + tt.json + `"op":"ping"}` + "\n"
			_, err := DecodeRequestV1(strings.NewReader(frame))
			requireProtocolV1VersionError(t, err, tt.version)
		})
		t.Run("decode response "+tt.name, func(t *testing.T) {
			frame := "{" + tt.json + `"op":"pong","ok":false,"ready":false,"warming":false}` + "\n"
			_, err := DecodeResponseV1(strings.NewReader(frame))
			requireProtocolV1VersionError(t, err, tt.version)
		})
	}
}

func TestProtocolV1CodecEnforcesFrameLimit(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		exact, exactJSON := protocolV1RequestWithJSONSize(t, MaxProtocolFrameBytes)
		var exactFrame bytes.Buffer
		if err := EncodeRequestV1(&exactFrame, exact); err != nil {
			t.Fatalf("encode exact-limit request: %v", err)
		}
		if got, want := exactFrame.Len(), MaxProtocolFrameBytes+1; got != want {
			t.Fatalf("exact-limit encoded length = %d, want %d", got, want)
		}
		if _, err := DecodeRequestV1(bytes.NewReader(exactFrame.Bytes())); err != nil {
			t.Fatalf("decode exact-limit request: %v", err)
		}
		crlf := append(append([]byte(nil), exactJSON...), '\r', '\n')
		if _, err := DecodeRequestV1(bytes.NewReader(crlf)); err != nil {
			t.Fatalf("decode exact-limit request with CRLF: %v", err)
		}

		oversized, oversizedJSON := protocolV1RequestWithJSONSize(t, MaxProtocolFrameBytes+1)
		var oversizedFrame bytes.Buffer
		if err := EncodeRequestV1(&oversizedFrame, oversized); err == nil {
			t.Fatal("encode oversized request succeeded")
		}
		if oversizedFrame.Len() != 0 {
			t.Fatalf("oversized request encode wrote %d bytes", oversizedFrame.Len())
		}
		reader := &protocolV1CountingReader{r: bytes.NewReader(append(append([]byte(nil), oversizedJSON...), '\n'))}
		if _, err := DecodeRequestV1(reader); err == nil {
			t.Fatal("decode oversized request succeeded")
		}
		requireProtocolV1BoundedRead(t, reader.n)
		unterminated := &protocolV1CountingReader{r: bytes.NewReader(bytes.Repeat([]byte{'x'}, MaxProtocolFrameBytes+4096))}
		if _, err := DecodeRequestV1(unterminated); err == nil {
			t.Fatal("decode oversized unterminated request succeeded")
		}
		requireProtocolV1BoundedRead(t, unterminated.n)
	})

	t.Run("response", func(t *testing.T) {
		exact, exactJSON := protocolV1ResponseWithJSONSize(t, MaxProtocolFrameBytes)
		var exactFrame bytes.Buffer
		if err := EncodeResponseV1(&exactFrame, exact); err != nil {
			t.Fatalf("encode exact-limit response: %v", err)
		}
		if got, want := exactFrame.Len(), MaxProtocolFrameBytes+1; got != want {
			t.Fatalf("exact-limit encoded length = %d, want %d", got, want)
		}
		if _, err := DecodeResponseV1(bytes.NewReader(exactFrame.Bytes())); err != nil {
			t.Fatalf("decode exact-limit response: %v", err)
		}
		crlf := append(append([]byte(nil), exactJSON...), '\r', '\n')
		if _, err := DecodeResponseV1(bytes.NewReader(crlf)); err != nil {
			t.Fatalf("decode exact-limit response with CRLF: %v", err)
		}

		oversized, oversizedJSON := protocolV1ResponseWithJSONSize(t, MaxProtocolFrameBytes+1)
		var oversizedFrame bytes.Buffer
		if err := EncodeResponseV1(&oversizedFrame, oversized); err == nil {
			t.Fatal("encode oversized response succeeded")
		}
		if oversizedFrame.Len() != 0 {
			t.Fatalf("oversized response encode wrote %d bytes", oversizedFrame.Len())
		}
		reader := &protocolV1CountingReader{r: bytes.NewReader(append(append([]byte(nil), oversizedJSON...), '\n'))}
		if _, err := DecodeResponseV1(reader); err == nil {
			t.Fatal("decode oversized response succeeded")
		}
		requireProtocolV1BoundedRead(t, reader.n)
		unterminated := &protocolV1CountingReader{r: bytes.NewReader(bytes.Repeat([]byte{'x'}, MaxProtocolFrameBytes+4096))}
		if _, err := DecodeResponseV1(unterminated); err == nil {
			t.Fatal("decode oversized unterminated response succeeded")
		}
		requireProtocolV1BoundedRead(t, unterminated.n)
	})
}

func TestProtocolV1CodecRequiresTerminatedNonEmptyFrame(t *testing.T) {
	validRequest, err := json.Marshal(RequestV1{Version: ProtocolVersionV1, Op: OpPing})
	if err != nil {
		t.Fatalf("marshal valid request fixture: %v", err)
	}
	validResponse, err := json.Marshal(ResponseV1{Version: ProtocolVersionV1, Op: OpPong})
	if err != nil {
		t.Fatalf("marshal valid response fixture: %v", err)
	}

	tests := []struct {
		name   string
		frame  []byte
		decode func(io.Reader) error
		valid  bool
	}{
		{name: "request empty reader", decode: protocolV1RequestDecodeError},
		{name: "request LF only", frame: []byte{'\n'}, decode: protocolV1RequestDecodeError},
		{name: "request CRLF only", frame: []byte{'\r', '\n'}, decode: protocolV1RequestDecodeError},
		{name: "request unterminated", frame: validRequest, decode: protocolV1RequestDecodeError},
		{name: "request LF", frame: append(append([]byte(nil), validRequest...), '\n'), decode: protocolV1RequestDecodeError, valid: true},
		{name: "request CRLF", frame: append(append([]byte(nil), validRequest...), '\r', '\n'), decode: protocolV1RequestDecodeError, valid: true},
		{name: "response empty reader", decode: protocolV1ResponseDecodeError},
		{name: "response LF only", frame: []byte{'\n'}, decode: protocolV1ResponseDecodeError},
		{name: "response CRLF only", frame: []byte{'\r', '\n'}, decode: protocolV1ResponseDecodeError},
		{name: "response unterminated", frame: validResponse, decode: protocolV1ResponseDecodeError},
		{name: "response LF", frame: append(append([]byte(nil), validResponse...), '\n'), decode: protocolV1ResponseDecodeError, valid: true},
		{name: "response CRLF", frame: append(append([]byte(nil), validResponse...), '\r', '\n'), decode: protocolV1ResponseDecodeError, valid: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.decode(bytes.NewReader(tt.frame))
			if tt.valid {
				if err != nil {
					t.Fatalf("decode valid frame: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("decode accepted unterminated or empty frame")
			}
		})
	}
}

func TestProtocolV1DecoderStopsAtFrameBoundary(t *testing.T) {
	t.Run("request", func(t *testing.T) {
		first := RequestV1{Version: ProtocolVersionV1, Op: OpPing, ID: "first"}
		second := RequestV1{Version: ProtocolVersionV1, Op: OpShutdown, ID: "second"}
		var frames bytes.Buffer
		if err := EncodeRequestV1(&frames, first); err != nil {
			t.Fatalf("encode first request: %v", err)
		}
		if err := EncodeRequestV1(&frames, second); err != nil {
			t.Fatalf("encode second request: %v", err)
		}
		reader := bufio.NewReader(&frames)
		gotFirst, err := DecodeRequestV1(reader)
		if err != nil {
			t.Fatalf("decode first request: %v", err)
		}
		gotSecond, err := DecodeRequestV1(reader)
		if err != nil {
			t.Fatalf("decode second request: %v", err)
		}
		if !reflect.DeepEqual(gotFirst, first) || !reflect.DeepEqual(gotSecond, second) {
			t.Fatalf("request frames changed: first=%#v second=%#v", gotFirst, gotSecond)
		}
	})

	t.Run("response", func(t *testing.T) {
		first := ResponseV1{Version: ProtocolVersionV1, Op: OpPong, ID: "first", OK: true, Ready: true}
		second := ResponseV1{Version: ProtocolVersionV1, Op: OpShutdownAck, ID: "second", OK: true}
		var frames bytes.Buffer
		if err := EncodeResponseV1(&frames, first); err != nil {
			t.Fatalf("encode first response: %v", err)
		}
		if err := EncodeResponseV1(&frames, second); err != nil {
			t.Fatalf("encode second response: %v", err)
		}
		reader := bufio.NewReader(&frames)
		gotFirst, err := DecodeResponseV1(reader)
		if err != nil {
			t.Fatalf("decode first response: %v", err)
		}
		gotSecond, err := DecodeResponseV1(reader)
		if err != nil {
			t.Fatalf("decode second response: %v", err)
		}
		if !reflect.DeepEqual(gotFirst, first) || !reflect.DeepEqual(gotSecond, second) {
			t.Fatalf("response frames changed: first=%#v second=%#v", gotFirst, gotSecond)
		}
	})
}

func TestProtocolV1DecodeRequiresPersistentByteReader(t *testing.T) {
	tests := []struct {
		name   string
		frame  string
		decode func(io.Reader) error
	}{
		{
			name:  "request",
			frame: `{"version":1,"op":"ping"}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeRequestV1(r)
				return err
			},
		},
		{
			name:  "response",
			frame: `{"version":1,"op":"pong","ok":true,"ready":true,"warming":false}` + "\n",
			decode: func(r io.Reader) error {
				_, err := DecodeResponseV1(r)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := &protocolV1ReadOnlySource{data: []byte(tt.frame), chunkSize: 7}
			err := tt.decode(source)
			if err == nil {
				t.Fatal("decode accepted a raw read-only source without a persistent byte reader")
			}
			if source.readCalls != 0 || source.bytesRead != 0 {
				t.Fatalf("decoder read raw source before rejecting it: calls=%d bytes=%d", source.readCalls, source.bytesRead)
			}
			message := strings.ToLower(err.Error())
			if !strings.Contains(message, "persistent") || !strings.Contains(message, "byte") || !strings.Contains(message, "reader") {
				t.Fatalf("decode error = %q, want persistent buffered/byte reader guidance", err)
			}
		})
	}
}

func TestProtocolV1BufferedDecoderPreservesChunkedSocketFrames(t *testing.T) {
	first := RequestV1{Version: ProtocolVersionV1, Op: OpPing, ID: "first"}
	second := RequestV1{Version: ProtocolVersionV1, Op: OpShutdown, ID: "second"}
	var frames bytes.Buffer
	if err := EncodeRequestV1(&frames, first); err != nil {
		t.Fatalf("encode first request: %v", err)
	}
	if err := EncodeRequestV1(&frames, second); err != nil {
		t.Fatalf("encode second request: %v", err)
	}

	source := &protocolV1ReadOnlySource{chunkSize: 7}
	source.Reset(frames.Bytes())
	reader := bufio.NewReaderSize(source, 32)
	gotFirst, err := DecodeRequestV1(reader)
	if err != nil {
		t.Fatalf("decode first request: %v", err)
	}
	gotSecond, err := DecodeRequestV1(reader)
	if err != nil {
		t.Fatalf("decode second request: %v", err)
	}
	if !reflect.DeepEqual(gotFirst, first) || !reflect.DeepEqual(gotSecond, second) {
		t.Fatalf("chunked request frames changed: first=%#v second=%#v", gotFirst, gotSecond)
	}
	if source.bytesRead != frames.Len() {
		t.Fatalf("buffered decoder read %d bytes, want exactly %d", source.bytesRead, frames.Len())
	}
	if source.maxRequested > reader.Size() {
		t.Fatalf("buffered decoder requested %d bytes, buffer size is %d", source.maxRequested, reader.Size())
	}
	if source.readCalls >= source.bytesRead {
		t.Fatalf("buffered decoder issued per-byte reads: calls=%d bytes=%d", source.readCalls, source.bytesRead)
	}
}

func BenchmarkProtocolV1EncodeRepresentative(b *testing.B) {
	request := representativeProtocolV1Request()
	var encoded bytes.Buffer
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded.Reset()
		if err := EncodeRequestV1(&encoded, request); err != nil {
			b.Fatal(err)
		}
	}
	benchmarkProtocolV1Bytes = encoded.Bytes()
}

func BenchmarkProtocolV1DecodeRepresentative(b *testing.B) {
	request := representativeProtocolV1Request()
	var encoded bytes.Buffer
	if err := EncodeRequestV1(&encoded, request); err != nil {
		b.Fatal(err)
	}
	frame := append([]byte(nil), encoded.Bytes()...)
	source := &protocolV1ReadOnlySource{chunkSize: 256}
	reader := bufio.NewReaderSize(source, 4096)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		source.Reset(frame)
		reader.Reset(source)
		got, err := DecodeRequestV1(reader)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkProtocolV1Request = got
	}
}

func BenchmarkProtocolV1RoundTripRepresentative(b *testing.B) {
	request := representativeProtocolV1Request()
	var encoded bytes.Buffer
	var reader bytes.Reader
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encoded.Reset()
		if err := EncodeRequestV1(&encoded, request); err != nil {
			b.Fatal(err)
		}
		reader.Reset(encoded.Bytes())
		got, err := DecodeRequestV1(&reader)
		if err != nil {
			b.Fatal(err)
		}
		benchmarkProtocolV1Request = got
	}
}

var (
	benchmarkProtocolV1Bytes   []byte
	benchmarkProtocolV1Request RequestV1
)

func representativeProtocolV1Request() RequestV1 {
	return RequestV1{
		Version: ProtocolVersionV1,
		Op:      OpRun,
		ID:      "request-17",
		Run: &RunRequestV1{
			Filter:              "Invoice*",
			ChangedSince:        "origin/main",
			SelectedClasses:     []string{"InvoiceTest", "TaxTest"},
			SelectedMethod:      "calculatesTotal",
			LimitMode:           "strict",
			LimitCaps:           distinctProtocolV1LimitCaps(),
			LimitCapsSet:        true,
			TraceBlocked:        true,
			TraceAll:            true,
			SlowTestThresholdMS: 250,
			TimeoutMS:           300000,
			Parallelism:         4,
			ParallelMethods:     true,
			NoDiskCache:         true,
			ClassDurationMS: map[string]int64{
				"InvoiceTest": 1200,
				"TaxTest":     800,
			},
			MethodDurationMS: map[string]int64{
				"InvoiceTest.calculatesTotal": 700,
				"TaxTest.appliesRate":         400,
			},
			PerfCounters:      true,
			ShardCount:        3,
			ShardIndex:        1,
			ReturnClassShards: true,
		},
	}
}

func distinctProtocolV1LimitCaps() LimitCapsV1 {
	return LimitCapsV1{
		Queries:             1,
		QueryRows:           2,
		DMLStatements:       3,
		DMLRows:             4,
		HeapSize:            5,
		CPUTimeMS:           6,
		Callouts:            7,
		AsyncJobs:           8,
		FutureCalls:         9,
		QueueableJobs:       10,
		BatchJobs:           11,
		ScheduledJobs:       12,
		EmailInvokes:        13,
		SOSLQueries:         14,
		QueryLocatorRows:    15,
		RunAs:               16,
		Savepoints:          17,
		SavepointRollbacks:  18,
		PublishImmediateDML: 19,
	}
}

func protocolV1LimitCapJSONKeys() []string {
	return []string{
		"queries",
		"queryRows",
		"dmlStatements",
		"dmlRows",
		"heapSize",
		"cpuTimeMs",
		"callouts",
		"asyncJobs",
		"futureCalls",
		"queueableJobs",
		"batchJobs",
		"scheduledJobs",
		"emailInvocations",
		"soslQueries",
		"queryLocatorRows",
		"runAs",
		"savepoints",
		"savepointRollbacks",
		"publishImmediateDml",
	}
}

func requireProtocolV1Map(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	value, ok := parent[key]
	if !ok {
		t.Fatalf("JSON object omitted %q", key)
	}
	child, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("JSON field %q = %#v, want object", key, value)
	}
	return child
}

func requireProtocolV1JSONValue(t *testing.T, object map[string]any, key string, want any) {
	t.Helper()
	got, ok := object[key]
	if !ok {
		t.Errorf("JSON object omitted %q", key)
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("JSON field %q = %#v, want %#v", key, got, want)
	}
}

func requireProtocolV1VersionError(t *testing.T, err error, version int) {
	t.Helper()
	if err == nil {
		t.Fatal("operation accepted invalid protocol version")
	}
	message := err.Error()
	if !strings.Contains(message, fmt.Sprint(version)) || !strings.Contains(message, fmt.Sprint(ProtocolVersionV1)) {
		t.Fatalf("version error = %q, want actual %d and supported %d", err, version, ProtocolVersionV1)
	}
}

func protocolV1RequestWithJSONSize(t *testing.T, target int) (RequestV1, []byte) {
	t.Helper()
	request := RequestV1{Version: ProtocolVersionV1, Op: OpPing, ID: "x"}
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request size fixture: %v", err)
	}
	filler := target - len(encoded) + 1
	if filler < 1 {
		t.Fatalf("target request JSON size %d is too small", target)
	}
	request.ID = strings.Repeat("x", filler)
	encoded, err = json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal sized request: %v", err)
	}
	if len(encoded) != target {
		t.Fatalf("sized request JSON length = %d, want %d", len(encoded), target)
	}
	return request, encoded
}

func protocolV1ResponseWithJSONSize(t *testing.T, target int) (ResponseV1, []byte) {
	t.Helper()
	response := ResponseV1{Version: ProtocolVersionV1, Op: OpPong, Project: "x"}
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response size fixture: %v", err)
	}
	filler := target - len(encoded) + 1
	if filler < 1 {
		t.Fatalf("target response JSON size %d is too small", target)
	}
	response.Project = strings.Repeat("x", filler)
	encoded, err = json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal sized response: %v", err)
	}
	if len(encoded) != target {
		t.Fatalf("sized response JSON length = %d, want %d", len(encoded), target)
	}
	return response, encoded
}

type protocolV1CountingReader struct {
	r io.Reader
	n int
}

func (r *protocolV1CountingReader) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	r.n += n
	return n, err
}

func (r *protocolV1CountingReader) ReadByte() (byte, error) {
	if reader, ok := r.r.(interface{ ReadByte() (byte, error) }); ok {
		b, err := reader.ReadByte()
		if err == nil {
			r.n++
		}
		return b, err
	}
	var data [1]byte
	n, err := r.Read(data[:])
	if n > 0 {
		return data[0], nil
	}
	return 0, err
}

type protocolV1ReadOnlySource struct {
	data         []byte
	offset       int
	chunkSize    int
	readCalls    int
	bytesRead    int
	maxRequested int
}

func (r *protocolV1ReadOnlySource) Reset(data []byte) {
	r.data = data
	r.offset = 0
	r.readCalls = 0
	r.bytesRead = 0
	r.maxRequested = 0
}

func (r *protocolV1ReadOnlySource) Read(p []byte) (int, error) {
	r.readCalls++
	if len(p) > r.maxRequested {
		r.maxRequested = len(p)
	}
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	if r.chunkSize > 0 && len(p) > r.chunkSize {
		p = p[:r.chunkSize]
	}
	n := copy(p, r.data[r.offset:])
	r.offset += n
	r.bytesRead += n
	return n, nil
}

func requireProtocolV1BoundedRead(t *testing.T, got int) {
	t.Helper()
	if wantMax := MaxProtocolFrameBytes + 2; got > wantMax {
		t.Fatalf("decoder read %d bytes from oversized frame, want at most %d", got, wantMax)
	}
}

func protocolV1RequestDecodeError(r io.Reader) error {
	_, err := DecodeRequestV1(r)
	return err
}

func protocolV1ResponseDecodeError(r io.Reader) error {
	_, err := DecodeResponseV1(r)
	return err
}
