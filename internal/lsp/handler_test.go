package lsp

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/schema"
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
	if !caps.DocumentSymbolProvider || !caps.WorkspaceSymbolProvider || !caps.HoverProvider || len(caps.CompletionProvider.TriggerCharacters) == 0 {
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
	for _, item := range items {
		labels = append(labels, item.Label)
		if item.Label == "Name" {
			t.Fatalf("field should not be a top-level completion item: %#v", item)
		}
	}
	want := []string{"Account", "InvoiceService", "PaymentGateway"}
	if strings.Join(labels, ",") != strings.Join(want, ",") {
		t.Fatalf("labels = %#v", labels)
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
