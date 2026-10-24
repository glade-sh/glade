package typesys

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestStandardSystemExceptionInheritance(t *testing.T) {
	symbols := StandardPlatformSymbols()
	for _, spec := range systemStubSymbolSpecs {
		if strings.Contains(spec.Name, ".") || !strings.HasSuffix(spec.Name, "Exception") {
			continue
		}
		t.Run(spec.Name, func(t *testing.T) {
			want := "Exception"
			if spec.Name == "Exception" {
				want = "Object"
			}
			if got := requireStandardSymbol(t, symbols, spec.Name).SuperClass; got != want {
				t.Fatalf("%s superclass=%s; want %s", spec.Name, got, want)
			}
		})
	}
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
			t.Run("nativeAPI"+api+"/"+row[0], func(t *testing.T) {
				for _, spelling := range []string{row[0], "System." + row[0], "system." + strings.ToUpper(row[0])} {
					parent, available := StandardExceptionParent(spelling)
					if available != (row[2] == "true") || parent != row[1] {
						t.Errorf("%s parent=(%q,%v); native=(%q,%s)", spelling, parent, available, row[1], row[2])
					}
				}
				if row[2] != "true" {
					return
				}
				symbol := requireStandardSymbol(t, symbols, row[0])
				if symbol.SuperClass != row[1] {
					t.Errorf("superclass=%s; native=%s", symbol.SuperClass, row[1])
				}
				extendable := slices.Contains(symbol.Modifiers, "virtual") || slices.Contains(symbol.Modifiers, "abstract")
				if extendable != (row[3] == "true") {
					t.Errorf("extendable=%v modifiers=%v; native=%s", extendable, symbol.Modifiers, row[3])
				}
			})
		}
	}
}
