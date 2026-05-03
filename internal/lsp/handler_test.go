package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/typesys"
)

func TestHandleJSONInitializeAndShutdown(t *testing.T) {
	handler := NewHandler(sampleIndex(t))

	data, err := handler.HandleJSON([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	var initialized struct {
		Result InitializeResult `json:"result"`
		Error  *ResponseError   `json:"error"`
	}
	if err := json.Unmarshal(data, &initialized); err != nil {
		t.Fatal(err)
	}
	if initialized.Error != nil {
		t.Fatalf("unexpected error: %#v", initialized.Error)
	}
	caps := initialized.Result.Capabilities
	if !caps.DocumentSymbolProvider || !caps.WorkspaceSymbolProvider || !caps.HoverProvider || !caps.DefinitionProvider || !caps.ReferencesProvider || !caps.RenameProvider.PrepareProvider || !caps.SemanticTokensProvider.Full || len(caps.CompletionProvider.TriggerCharacters) == 0 {
		t.Fatalf("capabilities = %#v", caps)
	}

	data, err = handler.HandleJSON([]byte(`{"jsonrpc":"2.0","id":2,"method":"shutdown"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !handler.Shutdown() {
		t.Fatal("shutdown flag not set")
	}
	var shutdown map[string]json.RawMessage
	if err := json.Unmarshal(data, &shutdown); err != nil {
		t.Fatal(err)
	}
	if string(shutdown["result"]) != "null" {
		t.Fatalf("shutdown result = %s", shutdown["result"])
	}
}

func TestBuildPublishDiagnosticsConvertsRanges(t *testing.T) {
	idx := sampleIndex(t)
	file := idx.Types[0].File
	diagRange := diagnostic.Range{
		Start: diagnostic.Position{Line: 2, Column: 3},
		End:   diagnostic.Position{Line: 2, Column: 12},
	}
	payload := BuildPublishDiagnostics(uriFromPath(file), []diagnostic.Diagnostic{
		{
			Severity: diagnostic.Error,
			Code:     "OAERTEST001",
			Message:  "broken",
			File:     file,
			Range:    &diagRange,
		},
		{
			Severity: diagnostic.Warning,
			Message:  "other file",
			File:     filepath.Join(filepath.Dir(file), "Other.cls"),
			Range:    &diagRange,
		},
	})

	if len(payload.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", payload.Diagnostics)
	}
	got := payload.Diagnostics[0]
	if got.Severity != diagnosticSeverityError || got.Code != "OAERTEST001" || got.Source != "oaer" {
		t.Fatalf("diagnostic = %#v", got)
	}
	if got.Range.Start.Line != 1 || got.Range.Start.Character != 2 || got.Range.End.Character != 11 {
		t.Fatalf("range = %#v", got.Range)
	}
}

func TestTextDocumentSyncPublishesOverlayDiagnostics(t *testing.T) {
	handler := NewHandler(sampleIndex(t))
	uri := uriFromPath(filepath.Join(t.TempDir(), "Draft.cls"))
	open := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{
		URI:     uri,
		Version: 1,
		Text:    "public class Draft {\n",
	}})
	if got := diagnosticsCount(t, open); got == 0 {
		t.Fatalf("open diagnostics = %d", got)
	}

	notifications, err := handler.DidChange(DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 2},
		ContentChanges: []TextDocumentContentChangeEvent{{
			Range: &Range{
				Start: Position{Line: 1, Character: 0},
				End:   Position{Line: 1, Character: 0},
			},
			Text: "}\n",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := diagnosticsCount(t, notifications); got != 0 {
		t.Fatalf("change diagnostics = %d", got)
	}

	closeNotifications := handler.DidClose(DidCloseTextDocumentParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	if got := diagnosticsCount(t, closeNotifications); got != 0 {
		t.Fatalf("close diagnostics = %d", got)
	}
}

func TestDidCloseRestoresProjectDiagnostics(t *testing.T) {
	idx := sampleIndex(t)
	file := idx.Types[0].File
	idx.Diagnostics = append(idx.Diagnostics, diagnostic.Diagnostic{
		Severity: diagnostic.Error,
		Code:     "OAERCHECK",
		Message:  "check diagnostic",
		File:     file,
		Range: &diagnostic.Range{
			Start: diagnostic.Position{Line: 1, Column: 1},
			End:   diagnostic.Position{Line: 1, Column: 2},
		},
	})
	handler := NewHandler(idx)
	uri := uriFromPath(file)
	handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{
		URI:  uri,
		Text: "public class InvoiceService {}\n",
	}})
	notifications := handler.DidClose(DidCloseTextDocumentParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	if got := diagnosticsCount(t, notifications); got != 1 {
		t.Fatalf("close diagnostics = %d", got)
	}
}

func TestPublishTestDiagnosticsUsesProblemStackFrames(t *testing.T) {
	file := filepath.Join(t.TempDir(), "InvoiceServiceTest.cls")
	handler := NewHandler(sampleIndex(t))
	notifications := handler.PublishTestDiagnostics(testreport.Run{Suites: []testreport.Suite{{
		Name: "InvoiceServiceTest",
		Cases: []testreport.Case{{
			ClassName:  "InvoiceServiceTest",
			MethodName: "fails",
			Status:     testreport.StatusFail,
			Problem: &testreport.Problem{
				Message: "Expected 1, got 2",
				Stack: []testreport.StackFrame{{
					Symbol: "InvoiceServiceTest.fails",
					File:   file,
					Line:   7,
					Column: 5,
				}},
			},
		}},
	}}})
	if len(notifications) != 1 {
		t.Fatalf("notifications = %#v", notifications)
	}
	payload, ok := notifications[0].Params.(PublishDiagnosticsParams)
	if !ok {
		t.Fatalf("params type = %T", notifications[0].Params)
	}
	if len(payload.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", payload.Diagnostics)
	}
	diag := payload.Diagnostics[0]
	if diag.Code != "OAERTEST001" || diag.Range.Start.Line != 6 || !strings.Contains(diag.Message, "InvoiceServiceTest.fails") {
		t.Fatalf("diagnostic = %#v", diag)
	}
}

func TestHandleJSONDocumentNotificationReturnsDiagnosticsNotification(t *testing.T) {
	handler := NewHandler(sampleIndex(t))
	uri := uriFromPath(filepath.Join(t.TempDir(), "Draft.cls"))
	data, err := handler.HandleJSON(mustJSON(t, map[string]any{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]any{
			"textDocument": map[string]any{
				"uri":     uri,
				"version": 1,
				"text":    "public class Draft {\n",
			},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}
	var notification Notification
	if err := json.Unmarshal(data, &notification); err != nil {
		t.Fatal(err)
	}
	if notification.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("notification = %#v", notification)
	}
}

func TestTextDocumentChangeRejectsInvalidRange(t *testing.T) {
	handler := NewHandler(sampleIndex(t))
	uri := uriFromPath(filepath.Join(t.TempDir(), "Draft.cls"))
	handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uri, Text: "public class Draft {}\n"}})
	_, err := handler.DidChange(DidChangeTextDocumentParams{
		TextDocument: VersionedTextDocumentIdentifier{URI: uri, Version: 2},
		ContentChanges: []TextDocumentContentChangeEvent{{
			Range: &Range{
				Start: Position{Line: 9, Character: 0},
				End:   Position{Line: 9, Character: 1},
			},
			Text: "x",
		}},
	})
	if err == nil {
		t.Fatal("expected invalid range error")
	}
}

func TestTextDocumentChangeUsesUTF16Positions(t *testing.T) {
	changed, err := applyTextChange("public class Draft { String s = 'Hi 😀 x'; }\n", TextDocumentContentChangeEvent{
		Range: &Range{
			Start: Position{Line: 0, Character: 39},
			End:   Position{Line: 0, Character: 40},
		},
		Text: "y",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(changed, "Hi 😀 y") {
		t.Fatalf("changed = %q", changed)
	}
	_, err = applyTextChange("Hi 😀 x\n", TextDocumentContentChangeEvent{
		Range: &Range{
			Start: Position{Line: 0, Character: 4},
			End:   Position{Line: 0, Character: 4},
		},
		Text: "bad",
	})
	if err == nil {
		t.Fatal("expected surrogate-pair split error")
	}
}

func TestTextDocumentChangeHandlesCRLFAndCRLineEndings(t *testing.T) {
	changed, err := applyTextChange("line1\r\nline2", TextDocumentContentChangeEvent{
		Range: &Range{
			Start: Position{Line: 1, Character: 0},
			End:   Position{Line: 1, Character: 5},
		},
		Text: "done",
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed != "line1\r\ndone" {
		t.Fatalf("changed = %q", changed)
	}
	changed, err = applyTextChange("line1\rline2", TextDocumentContentChangeEvent{
		Range: &Range{
			Start: Position{Line: 1, Character: 0},
			End:   Position{Line: 1, Character: 5},
		},
		Text: "done",
	})
	if err != nil {
		t.Fatal(err)
	}
	if changed != "line1\rdone" {
		t.Fatalf("changed = %q", changed)
	}
}

func TestDocumentSymbolsFromIndexFile(t *testing.T) {
	idx := sampleIndex(t)
	symbols := NewHandler(idx).DocumentSymbols(uriFromPath(idx.Types[0].File))

	if len(symbols) != 1 {
		t.Fatalf("symbols = %#v", symbols)
	}
	if symbols[0].Name != "InvoiceService" || symbols[0].Kind != symbolKindClass {
		t.Fatalf("type symbol = %#v", symbols[0])
	}
	if len(symbols[0].Children) != 2 || symbols[0].Children[0].Name != "account" || symbols[0].Children[1].Name != "run" {
		t.Fatalf("children = %#v", symbols[0].Children)
	}
	if symbols[0].Range.Start.Line != 0 || symbols[0].Range.Start.Character != 0 {
		t.Fatalf("range = %#v", symbols[0].Range)
	}
}

func TestWorkspaceSymbolsIncludeApexSchemaAndFields(t *testing.T) {
	symbols := NewHandler(sampleIndex(t)).WorkspaceSymbols("name")

	if len(symbols) != 1 {
		t.Fatalf("symbols = %#v", symbols)
	}
	if symbols[0].Name != "Name" || symbols[0].ContainerName != "Account" || symbols[0].Kind != symbolKindField {
		t.Fatalf("field symbol = %#v", symbols[0])
	}

	account := NewHandler(sampleIndex(t)).WorkspaceSymbols("acc")
	if len(account) != 1 || account[0].Name != "Account" || account[0].Kind != symbolKindObject {
		t.Fatalf("account symbols = %#v", account)
	}
}

func TestHoverForKnownTypes(t *testing.T) {
	idx := sampleIndex(t)
	handler := NewHandler(idx)

	apexHover := handler.HoverForName("InvoiceService", nil)
	if apexHover == nil || !strings.Contains(apexHover.Contents.Value, "class InvoiceService") {
		t.Fatalf("apex hover = %#v", apexHover)
	}

	builtinHover := handler.HoverForName("String", nil)
	if builtinHover == nil || !strings.Contains(builtinHover.Contents.Value, "Builtin type") {
		t.Fatalf("builtin hover = %#v", builtinHover)
	}

	positionHover := handler.Hover(HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uriFromPath(idx.Types[0].File)},
		Position:     Position{Line: 1, Character: 8},
	})
	if positionHover == nil || !strings.Contains(positionHover.Contents.Value, "Account") || !strings.Contains(positionHover.Contents.Value, "SObject") {
		t.Fatalf("position hover = %#v", positionHover)
	}
}

func TestCompletionIncludesTopLevelApexTypesAndSObjects(t *testing.T) {
	items := NewHandler(sampleIndex(t)).Completion(CompletionParams{}).Items

	var labels []string
	foundField := false
	foundMember := false
	foundKeyword := false
	for _, item := range items {
		labels = append(labels, item.Label)
		if item.Label == "Name" && item.Detail == "Account.Name" {
			foundField = true
		}
		if item.Label == "run" && item.Detail == "InvoiceService.run" {
			foundMember = true
		}
		if item.Label == "trigger" && item.Kind == completionItemKindKeyword {
			foundKeyword = true
		}
	}
	for _, want := range []string{"Account", "InvoiceService", "PaymentGateway"} {
		if !containsString(labels, want) {
			t.Fatalf("missing %s in %#v", want, labels)
		}
	}
	if !foundField || !foundMember || !foundKeyword {
		t.Fatalf("labels = %#v", labels)
	}
}

func TestDefinitionReferencesRenameAndSemanticTokens(t *testing.T) {
	idx := sampleIndex(t)
	writeSampleSources(t, idx)
	handler := NewHandler(idx)
	uri := uriFromPath(idx.Types[0].File)

	definition := handler.Definition(DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 15},
	})
	if len(definition) != 1 || definition[0].Range.Start.Line != 0 {
		t.Fatalf("definition = %#v", definition)
	}

	references := handler.References(ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 15},
		Context:      ReferenceContext{IncludeDeclaration: true},
	})
	if len(references) != 3 || references[2].Range.Start.Character != 40 {
		t.Fatalf("references = %#v", references)
	}

	edit := handler.Rename(RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 15},
		NewName:      "InvoiceRunner",
	})
	if edit == nil || len(edit.Changes[uri]) != 3 {
		t.Fatalf("rename = %#v", edit)
	}

	tokens := handler.SemanticTokensFull(SemanticTokensParams{TextDocument: TextDocumentIdentifier{URI: uri}})
	if len(tokens.Data) == 0 || len(tokens.Data)%5 != 0 {
		t.Fatalf("tokens = %#v", tokens.Data)
	}
}

func TestHandleRequestDispatchesWorkspaceSymbols(t *testing.T) {
	handler := NewHandler(sampleIndex(t))
	resp := handler.HandleRequest(Request{
		ID:     json.RawMessage(`3`),
		Method: "workspace/symbol",
		Params: json.RawMessage(`{"query":"pay"}`),
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %#v", resp.Error)
	}
	symbols, ok := resp.Result.([]SymbolInformation)
	if !ok {
		t.Fatalf("result type = %T", resp.Result)
	}
	if len(symbols) != 1 || symbols[0].Name != "PaymentGateway" {
		t.Fatalf("symbols = %#v", symbols)
	}
}

func sampleIndex(t *testing.T) typesys.Index {
	t.Helper()
	root := t.TempDir()
	serviceFile := filepath.Join(root, "InvoiceService.cls")
	gatewayFile := filepath.Join(root, "PaymentGateway.cls")
	triggerFile := filepath.Join(root, "InvoiceTrigger.trigger")
	return typesys.Index{
		Project: typesys.ProjectInfo{Root: root},
		Types: []typesys.TypeSymbol{
			{
				Kind: apexast.DeclarationClass,
				Name: "InvoiceService",
				File: serviceFile,
				Range: diagnostic.Range{
					Start: diagnostic.Position{Line: 1, Column: 1},
					End:   diagnostic.Position{Line: 5, Column: 2},
				},
				Members: []typesys.MemberSymbol{
					{
						Kind: apexast.DeclarationField,
						Name: "account",
						Type: "Account",
						Range: diagnostic.Range{
							Start: diagnostic.Position{Line: 2, Column: 3},
							End:   diagnostic.Position{Line: 2, Column: 19},
						},
					},
					{
						Kind: apexast.DeclarationMethod,
						Name: "run",
						Type: "void",
						Range: diagnostic.Range{
							Start: diagnostic.Position{Line: 3, Column: 3},
							End:   diagnostic.Position{Line: 3, Column: 22},
						},
					},
				},
			},
			{
				Kind: apexast.DeclarationInterface,
				Name: "PaymentGateway",
				File: gatewayFile,
				Range: diagnostic.Range{
					Start: diagnostic.Position{Line: 1, Column: 1},
					End:   diagnostic.Position{Line: 1, Column: 34},
				},
			},
		},
		Triggers: []typesys.TriggerSymbol{
			{
				Name:       "InvoiceTrigger",
				ObjectName: "Account",
				File:       triggerFile,
				Range: diagnostic.Range{
					Start: diagnostic.Position{Line: 1, Column: 1},
					End:   diagnostic.Position{Line: 1, Column: 51},
				},
			},
		},
		Objects: []schema.Object{
			{
				Name:  "Account",
				Label: "Account",
				Fields: []schema.Field{
					{Name: "Name", Type: "Text"},
				},
			},
		},
	}
}

func writeSampleSources(t *testing.T, idx typesys.Index) {
	t.Helper()
	if err := os.WriteFile(idx.Types[0].File, []byte(`public class InvoiceService {
  Account account;
  void run() { InvoiceService svc = new invoiceservice(); account = new Account(); }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(idx.Types[1].File, []byte(`public interface PaymentGateway {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(idx.Triggers[0].File, []byte(`trigger InvoiceTrigger on Account (before insert) {}
`), 0o644); err != nil {
		t.Fatal(err)
	}
}

func diagnosticsCount(t *testing.T, notifications []Notification) int {
	t.Helper()
	if len(notifications) != 1 {
		t.Fatalf("notifications = %#v", notifications)
	}
	payload, ok := notifications[0].Params.(PublishDiagnosticsParams)
	if !ok {
		t.Fatalf("params type = %T", notifications[0].Params)
	}
	return len(payload.Diagnostics)
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
