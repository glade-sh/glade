package lsp

import (
	"context"
	"encoding/json"
	"errors"
)

type Request struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	JSONRPC string
	ID      json.RawMessage
	Result  any
	Error   *ResponseError
}

type ResponseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

const (
	errorCodeParseError       = -32700
	errorCodeInvalidRequest   = -32600
	errorCodeMethodNotFound   = -32601
	errorCodeInvalidParams    = -32602
	errorCodeInternalError    = -32603
	errorCodeRequestCancelled = -32800
)

func (h *Handler) HandleJSON(data []byte) ([]byte, error) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		resp := Response{
			JSONRPC: "2.0",
			Error:   &ResponseError{Code: errorCodeParseError, Message: err.Error()},
		}
		return json.Marshal(resp)
	}
	if len(req.ID) == 0 {
		result, rpcErr := h.handle(req.Method, req.Params)
		if rpcErr != nil {
			return nil, nil
		}
		if notifications, ok := result.([]Notification); ok && len(notifications) == 1 {
			return json.Marshal(notifications[0])
		}
		return nil, nil
	}
	return json.Marshal(h.HandleRequest(req))
}

func (h *Handler) HandleRequest(req Request) Response {
	result, rpcErr := h.handle(req.Method, req.Params)
	return Response{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
		Error:   rpcErr,
	}
}

func (h *Handler) handle(method string, params json.RawMessage) (any, *ResponseError) {
	return h.handleContext(context.Background(), method, params)
}

// indexed runs fn once the project index is ready. It returns early with a
// cancellation error if ctx ends first, whether while waiting for the index or
// while fn runs. An abandoned fn finishes in the background and its result is
// dropped.
func (h *Handler) indexed(ctx context.Context, fn func() any) (any, *ResponseError) {
	if err := h.waitReady(ctx); err != nil {
		return nil, indexError(err)
	}
	if ctx.Done() == nil {
		return fn(), nil
	}
	if err := ctx.Err(); err != nil {
		return nil, indexError(err)
	}
	done := make(chan any, 1)
	go func() { done <- fn() }()
	select {
	case result := <-done:
		return result, nil
	case <-ctx.Done():
		return nil, indexError(ctx.Err())
	}
}

func indexError(err error) *ResponseError {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &ResponseError{Code: errorCodeRequestCancelled, Message: "request cancelled"}
	}
	return &ResponseError{Code: errorCodeInternalError, Message: "project index unavailable: " + err.Error()}
}

func (h *Handler) handleContext(ctx context.Context, method string, params json.RawMessage) (any, *ResponseError) {
	switch method {
	case "initialize":
		var p InitializeParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.Initialize(p), nil
	case "shutdown":
		h.shutdown.Store(true)
		h.parserMu.Lock()
		h.parser.Close()
		h.parserMu.Unlock()
		return nil, nil
	case "textDocument/didOpen":
		var p DidOpenTextDocumentParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.DidOpen(p), nil
	case "textDocument/didChange":
		var p DidChangeTextDocumentParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		notifications, err := h.DidChange(p)
		if err != nil {
			return nil, invalidParams(err)
		}
		return notifications, nil
	case "textDocument/didClose":
		var p DidCloseTextDocumentParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.DidClose(p), nil
	case "textDocument/documentSymbol":
		var p DocumentSymbolParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.indexed(ctx, func() any { return h.DocumentSymbols(p.TextDocument.URI) })
	case "workspace/symbol":
		var p WorkspaceSymbolParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.indexed(ctx, func() any { return h.WorkspaceSymbols(p.Query) })
	case "textDocument/hover":
		var p HoverParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.indexed(ctx, func() any { return h.Hover(p) })
	case "textDocument/completion":
		var p CompletionParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.indexed(ctx, func() any { return h.Completion(p) })
	case "textDocument/definition":
		var p DefinitionParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.indexed(ctx, func() any { return h.Definition(p) })
	case "textDocument/references":
		var p ReferenceParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.indexed(ctx, func() any { return h.References(p) })
	case "textDocument/prepareRename":
		var p RenameParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.indexed(ctx, func() any { return h.PrepareRename(p) })
	case "textDocument/rename":
		var p RenameParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.indexed(ctx, func() any { return h.Rename(p) })
	case "textDocument/semanticTokens/full":
		var p SemanticTokensParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.indexed(ctx, func() any { return h.SemanticTokensFull(p) })
	default:
		return nil, &ResponseError{Code: errorCodeMethodNotFound, Message: "method not found"}
	}
}

func decodeParams(data json.RawMessage, out any) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return json.Unmarshal(data, out)
}

func invalidParams(err error) *ResponseError {
	return &ResponseError{Code: errorCodeInvalidParams, Message: err.Error()}
}

func (r Response) MarshalJSON() ([]byte, error) {
	out := map[string]any{"jsonrpc": r.JSONRPC}
	if out["jsonrpc"] == "" {
		out["jsonrpc"] = "2.0"
	}
	if len(r.ID) > 0 {
		out["id"] = r.ID
	}
	if r.Error != nil {
		out["error"] = r.Error
	} else {
		out["result"] = r.Result
	}
	return json.Marshal(out)
}
