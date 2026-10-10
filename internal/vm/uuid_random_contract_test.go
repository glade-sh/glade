package vm

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"reflect"
	"testing"
)

// Current-candidate public API 67 observer. The final state check is a
// deterministic white-box source-path sentinel, not a public no-collision or
// statistical entropy assertion. It should fail while UUID.randomUUID consumes
// the VM's deterministic stream.
const uuidRandomRedApexSource = `UUID generated = UUID.randomUUID();
String rendered = generated.toString();
System.assert(generated != null);
System.assertEquals(36, rendered.length());
System.assertEquals('-', rendered.substring(8, 9));
System.assertEquals('-', rendered.substring(13, 14));
System.assertEquals('-', rendered.substring(18, 19));
System.assertEquals('-', rendered.substring(23, 24));
System.assertEquals('4', rendered.substring(14, 15));
System.assert('89ab'.contains(rendered.substring(19, 20)));`

func TestUUIDRandomCurrentSourceAPI67(t *testing.T) {
	program, compileErr := CompileAnonymousWithOptions(uuidRandomRedApexSource, CompileOptions{APIVersion: "67.0"})
	if compileErr != nil {
		t.Fatalf("step=compile api=67.0 source=%q compileErr=%v", uuidRandomRedApexSource, compileErr)
	}
	if program.APIVersion != "67.0" {
		t.Fatalf("step=program-api-guard got=%q want=67.0 source=%q", program.APIVersion, uuidRandomRedApexSource)
	}

	machine := New(nil)
	before := machine.DeterministicRandomState()
	result, executeErr := machine.Execute(program)
	after := machine.DeterministicRandomState()
	if executeErr != nil {
		t.Fatalf("step=execute api=67.0 source=%q result=%#v executeErr=%v deterministicStateBefore=%#x deterministicStateAfter=%#x", uuidRandomRedApexSource, result, executeErr, before, after)
	}
	if after != before {
		t.Fatalf("step=secure-source-red api=67.0 source=%q result=%#v deterministicStateBefore=%#x deterministicStateAfter=%#x; public UUID.randomUUID consumed the deterministic VM stream", uuidRandomRedApexSource, result, before, after)
	}
}

const uuidRandomGreenApexSource = "UUID generated = UUID.randomUUID();\nString rendered = generated.toString();\nSystem.assertEquals('00010203-0405-4607-8809-0a0b0c0d0e0f', rendered);"

func TestUUIDRandomDefaultSourceUsesCryptoRandAPI67(t *testing.T) {
	machine := New(nil)
	if got := machine.uuidRandomSource(); !reflect.DeepEqual(got, rand.Reader) {
		t.Fatalf("default UUID reader = %T %#v, want crypto/rand.Reader %T %#v", got, got, rand.Reader, rand.Reader)
	}
}

func TestUUIDRandomInjectedReaderPublicAPI67(t *testing.T) {
	input := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	reader := &uuidRandomCountingReader{reader: bytes.NewReader(input), chunkSize: 5}
	machine := New(nil)
	machine.uuidRandomReader = reader
	before := machine.DeterministicRandomState()

	program, compileErr := CompileAnonymousWithOptions(uuidRandomGreenApexSource, CompileOptions{APIVersion: "67.0"})
	if compileErr != nil {
		t.Fatalf("step=compile api=67.0 source=%q compileErr=%v", uuidRandomGreenApexSource, compileErr)
	}
	if program.APIVersion != "67.0" {
		t.Fatalf("step=program-api-guard got=%q want=67.0 source=%q", program.APIVersion, uuidRandomGreenApexSource)
	}
	result, executeErr := machine.Execute(program)
	if executeErr != nil {
		t.Fatalf("step=execute api=67.0 source=%q result=%#v executeErr=%v bytesRead=%d reads=%d", uuidRandomGreenApexSource, result, executeErr, reader.bytesRead, reader.reads)
	}
	if reader.bytesRead != 16 || reader.reader.Len() != 0 {
		t.Fatalf("secure source bytesRead=%d unread=%d, want 16 bytes and none unread", reader.bytesRead, reader.reader.Len())
	}
	if reader.reads < 2 {
		t.Fatalf("secure source reads=%d, want multiple short reads to be completed", reader.reads)
	}
	if after := machine.DeterministicRandomState(); after != before {
		t.Fatalf("UUID.randomUUID changed deterministic VM state: before=%#x after=%#x", before, after)
	}
}

func TestUUIDRandomReaderFailureDoesNotFallbackAPI67(t *testing.T) {
	machine := New(nil)
	machine.uuidRandomReader = uuidRandomFailingReader{}
	before := machine.DeterministicRandomState()
	program, compileErr := CompileAnonymousWithOptions("UUID generated = UUID.randomUUID();", CompileOptions{APIVersion: "67.0"})
	if compileErr != nil {
		t.Fatalf("step=compile api=67.0 compileErr=%v", compileErr)
	}
	if program.APIVersion != "67.0" {
		t.Fatalf("step=program-api-guard got=%q want=67.0", program.APIVersion)
	}
	result, executeErr := machine.Execute(program)
	if !errors.Is(executeErr, errUUIDRandomReaderFailure) {
		t.Fatalf("step=entropy-error expected=%v result=%#v executeErr=%v", errUUIDRandomReaderFailure, result, executeErr)
	}
	if after := machine.DeterministicRandomState(); after != before {
		t.Fatalf("reader failure fell through to deterministic VM state: before=%#x after=%#x", before, after)
	}
}

func TestUUIDRandomTruncatedReaderReturnsUnexpectedEOFAPI67(t *testing.T) {
	machine := New(nil)
	reader := &uuidRandomCountingReader{reader: bytes.NewReader(make([]byte, 15)), chunkSize: 4}
	machine.uuidRandomReader = reader
	before := machine.DeterministicRandomState()
	program, compileErr := CompileAnonymousWithOptions("UUID generated = UUID.randomUUID();", CompileOptions{APIVersion: "67.0"})
	if compileErr != nil {
		t.Fatalf("step=compile api=67.0 compileErr=%v", compileErr)
	}
	if program.APIVersion != "67.0" {
		t.Fatalf("step=program-api-guard got=%q want=67.0", program.APIVersion)
	}
	result, executeErr := machine.Execute(program)
	if !errors.Is(executeErr, io.ErrUnexpectedEOF) {
		t.Fatalf("step=short-read expected=%v result=%#v executeErr=%v bytesRead=%d", io.ErrUnexpectedEOF, result, executeErr, reader.bytesRead)
	}
	if reader.bytesRead != 15 {
		t.Fatalf("short source bytesRead=%d, want 15", reader.bytesRead)
	}
	if after := machine.DeterministicRandomState(); after != before {
		t.Fatalf("truncated reader fell through to deterministic VM state: before=%#x after=%#x", before, after)
	}
}

type uuidRandomCountingReader struct {
	reader    *bytes.Reader
	chunkSize int
	bytesRead int
	reads     int
}

func (r *uuidRandomCountingReader) Read(p []byte) (int, error) {
	if r.chunkSize > 0 && len(p) > r.chunkSize {
		p = p[:r.chunkSize]
	}
	n, err := r.reader.Read(p)
	r.bytesRead += n
	r.reads++
	return n, err
}

var errUUIDRandomReaderFailure = errors.New("controlled UUID entropy read failure")

type uuidRandomFailingReader struct{}

func (uuidRandomFailingReader) Read([]byte) (int, error) {
	return 0, errUUIDRandomReaderFailure
}
