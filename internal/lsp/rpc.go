package lsp

import (
	"encoding/json"
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
	errorCodeParseError     = -32700
	errorCodeInvalidRequest = -32600
	errorCodeMethodNotFound = -32601
	errorCodeInvalidParams  = -32602
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
	switch method {
	case "initialize":
		var p InitializeParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.Initialize(p), nil
	case "shutdown":
		h.shutdown = true
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
		return h.DocumentSymbols(p.TextDocument.URI), nil
	case "workspace/symbol":
		var p WorkspaceSymbolParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.WorkspaceSymbols(p.Query), nil
	case "textDocument/hover":
		var p HoverParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.Hover(p), nil
	case "textDocument/completion":
		var p CompletionParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.Completion(p), nil
	case "textDocument/definition":
		var p DefinitionParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.Definition(p), nil
	case "textDocument/references":
		var p ReferenceParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.References(p), nil
	case "textDocument/prepareRename":
		var p RenameParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.PrepareRename(p), nil
	case "textDocument/rename":
		var p RenameParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.Rename(p), nil
	case "textDocument/semanticTokens/full":
		var p SemanticTokensParams
		if err := decodeParams(params, &p); err != nil {
			return nil, invalidParams(err)
		}
		return h.SemanticTokensFull(p), nil
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
