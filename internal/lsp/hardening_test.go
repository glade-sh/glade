package lsp

import (
	"encoding/json"
	"testing"
)

func TestNoPanicOnMalformedLSPRequests(t *testing.T) {
	handler := NewHandler(benchmarkLSPIndex(5))
	requests := [][]byte{
		[]byte(`{`),
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"workspace/symbol","params":`),
		[]byte(`{"jsonrpc":"2.0","id":2,"method":"unknown","params":{}}`),
	}
	for _, request := range requests {
		assertNoPanic(t, func() {
			_, _ = handler.HandleJSON(request)
		})
	}
	assertNoPanic(t, func() {
		_ = handler.HandleRequest(Request{
			ID:     json.RawMessage(`3`),
			Method: "textDocument/hover",
			Params: json.RawMessage(`{"textDocument":{"uri":"file:///missing.cls"},"position":{"line":-1,"character":-1}}`),
		})
	})
}

func assertNoPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()
	fn()
}
