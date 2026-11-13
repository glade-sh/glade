package visualforce_test

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"
)

// The planner must select exactly the API passes executed by each parent.
func TestConformanceAPIShardManifest(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance_api_shards.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string][]string
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	families := []string{"v04", "v09", "v11", "v13", "v14"}
	want := map[string][]string{}
	for _, family := range families {
		file, err := parser.ParseFile(token.NewFileSet(), family+"_conformance_test.go", nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		variable := family + "APIVersions"
		var apis []string
		ast.Inspect(file, func(n ast.Node) bool {
			spec, ok := n.(*ast.ValueSpec)
			if !ok || len(spec.Names) != 1 || spec.Names[0].Name != variable || len(spec.Values) != 1 {
				return true
			}
			literal, ok := spec.Values[0].(*ast.CompositeLit)
			if !ok {
				return true
			}
			for _, element := range literal.Elts {
				value, ok := element.(*ast.BasicLit)
				if !ok {
					t.Fatalf("%s API must be a literal", variable)
				}
				var api string
				if err := json.Unmarshal([]byte(value.Value), &api); err != nil {
					t.Fatal(err)
				}
				apis = append(apis, api)
			}
			return false
		})
		// The family's parent tests, and the function that holds the API loop.
		// V09 and V14 are split by row range into top-level parts that share one runner.
		parents, runner := []string{"TestV" + family[1:] + "SalesforceConformance"}, ""
		if family == "v09" || family == "v14" {
			partCount := 4
			if family == "v14" {
				partCount = 2
			}
			parents, runner = nil, family+"RunConformance"
			for part := 1; part <= partCount; part++ {
				parents = append(parents, fmt.Sprintf("TestV%sSalesforceConformancePart%dof%d", family[1:], part, partCount))
			}
		}
		parent := runner
		if runner == "" {
			parent = parents[0]
		}
		if len(apis) == 0 {
			t.Fatalf("%s has no API declaration", parent)
		}
		sharded := false
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != parent {
				continue
			}
			ast.Inspect(function.Body, func(n ast.Node) bool {
				loop, ok := n.(*ast.RangeStmt)
				if !ok {
					return true
				}
				name, ok := loop.X.(*ast.Ident)
				if !ok || name.Name != variable {
					return true
				}
				ast.Inspect(loop.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok || len(call.Args) != 2 {
						return true
					}
					selector, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || selector.Sel.Name != "Run" {
						return true
					}
					api, ok := call.Args[0].(*ast.Ident)
					iterator, iterOK := loop.Value.(*ast.Ident)
					if ok && iterOK && api.Name == iterator.Name {
						sharded = true
					}
					return true
				})
				return true
			})
		}
		if !sharded {
			t.Fatalf("%s must run each declared API with t.Run(api)", parent)
		}
		for _, name := range parents {
			want[name] = apis
		}
		if runner != "" {
			for _, name := range parents {
				calls := false
				for _, declaration := range file.Decls {
					function, ok := declaration.(*ast.FuncDecl)
					if !ok || function.Name.Name != name {
						continue
					}
					ast.Inspect(function.Body, func(n ast.Node) bool {
						if call, ok := n.(*ast.CallExpr); ok {
							if identifier, ok := call.Fun.(*ast.Ident); ok && identifier.Name == runner {
								calls = true
							}
						}
						return true
					})
				}
				if !calls {
					t.Fatalf("%s must run %s", name, runner)
				}
			}
		}
	}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("API shard manifest = %v, want %v", manifest, want)
	}
}
