package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSystemExceptionNativeExtensionDeclarations(t *testing.T) {
	for _, api := range []string{"62", "67"} {
		file, err := os.Open(filepath.Join("..", "apextest", "testdata", "conformance", "system_exception_hierarchy", "native", "api"+api, "hierarchy.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		reader := csv.NewReader(file)
		reader.Comma = '\t'
		rows, err := reader.ReadAll()
		file.Close()
		if err != nil || len(rows) != 53 {
			t.Fatalf("incomplete native hierarchy: %v", err)
		}
		for _, row := range rows[1:] {
			if row[2] != "true" {
				continue
			}
			for _, spelling := range []string{row[0], "System." + row[0]} {
				t.Run(api+"/namedDeclaration/"+spelling, func(t *testing.T) {
					index := typesys.Index{Types: []typesys.TypeSymbol{{Name: "LarchChildException", Kind: apexast.DeclarationClass, SuperClass: spelling, EffectiveAPIVersion: api + ".0"}}}
					diagnostics := checkExceptionDeclarations(index)
					rejected := false
					for _, d := range diagnostics {
						rejected = rejected || strings.Contains(d.NativeMessage, "type cannot be extended")
					}
					if rejected != (row[3] == "false") {
						t.Fatalf("extension rejected=%v; native extendable=%s; diagnostics=%v", rejected, row[3], diagnostics)
					}
				})
			}
		}
	}
}
