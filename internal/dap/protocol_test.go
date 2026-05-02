package dap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestWriteAndReadDAPMessages(t *testing.T) {
	first := Request{Seq: 1, Type: MessageTypeRequest, Command: CommandThreads}
	second := Response{Seq: 1, Type: MessageTypeResponse, RequestSeq: 1, Success: true, Command: CommandThreads}

	var stream bytes.Buffer
	if err := Write(&stream, first); err != nil {
		t.Fatal(err)
	}
	if err := Write(&stream, second); err != nil {
		t.Fatal(err)
	}

	reader := NewReader(&stream)
	raw, err := reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	var gotFirst Request
	if err := json.Unmarshal(raw, &gotFirst); err != nil {
		t.Fatal(err)
	}
	if gotFirst.Command != CommandThreads || gotFirst.Seq != 1 {
		t.Fatalf("first message = %#v", gotFirst)
	}

	raw, err = reader.Read()
	if err != nil {
		t.Fatal(err)
	}
	var gotSecond Response
	if err := json.Unmarshal(raw, &gotSecond); err != nil {
		t.Fatal(err)
	}
	if gotSecond.Type != MessageTypeResponse || !gotSecond.Success || gotSecond.RequestSeq != 1 {
		t.Fatalf("second message = %#v", gotSecond)
	}
}

func TestReadRequestRejectsNonRequest(t *testing.T) {
	var stream bytes.Buffer
	if err := Write(&stream, Response{Seq: 1, Type: MessageTypeResponse, Success: true}); err != nil {
		t.Fatal(err)
	}
	_, err := NewReader(&stream).ReadRequest()
	if err == nil || !strings.Contains(err.Error(), "expected DAP request") {
		t.Fatalf("err = %v", err)
	}
}

func TestReadRejectsMissingContentLength(t *testing.T) {
	_, err := NewReader(strings.NewReader("X-Test: ok\r\n\r\n{}")).Read()
	if err == nil || !strings.Contains(err.Error(), "missing DAP Content-Length") {
		t.Fatalf("err = %v", err)
	}
}

func TestEncodeUsesContentLength(t *testing.T) {
	message := Request{Seq: 7, Type: MessageTypeRequest, Command: CommandPause}
	encoded, err := Encode(message)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	wantHeader := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if !bytes.HasPrefix(encoded, []byte(wantHeader)) {
		t.Fatalf("encoded prefix = %q, want %q", encoded, wantHeader)
	}
	if gotBody := encoded[len(wantHeader):]; !bytes.Equal(gotBody, body) {
		t.Fatalf("body = %s, want %s", gotBody, body)
	}
}
