package lsp

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func BenchmarkWorkspaceSymbols(b *testing.B) {
	handler := NewHandler(benchmarkLSPIndex(200))
	request := Request{
		ID:     json.RawMessage(`1`),
		Method: "workspace/symbol",
		Params: json.RawMessage(`{"query":"acc"}`),
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		response := handler.HandleRequest(request)
		if response.Error != nil {
			b.Fatalf("response = %#v", response)
		}
	}
}

func benchmarkLSPIndex(classes int) typesys.Index {
	index := typesys.Index{
		Objects: []schema.Object{{
			Name:  "Account",
			Label: "Account",
			Fields: []schema.Field{
				{Name: "Name", Type: "Text"},
			},
		}},
	}
	for i := 0; i < classes; i++ {
		index.Types = append(index.Types, typesys.TypeSymbol{
			Kind: apexast.DeclarationClass,
			Name: fmt.Sprintf("AccountService%03d", i),
			File: fmt.Sprintf("AccountService%03d.cls", i),
		})
	}
	return index
}
