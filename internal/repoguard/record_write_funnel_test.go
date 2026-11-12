package repoguard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// recordWriteFunnelExempt lists files allowed to write ObjectState.Records
// directly. storage/record_writes.go holds the funnel itself.
var recordWriteFunnelExempt = []string{
	"internal/storage/record_writes.go",
}

// recordsFieldOwners names receivers whose Records field is not
// storage.ObjectState.Records: JSON payload structs, report summaries and
// slices. The check is syntactic, so these are matched by file and receiver
// expression.
var recordsFieldOwners = map[string][]string{
	"internal/server/tooling_metadata_bulk.go": {"result"},
	"internal/server/data_query_handlers.go":   {"result"},
	"internal/gladecli/db_command.go":          {"result"},
	"internal/dbmanager/lookup.go":             {"out"},
	"internal/dml/automation.go":               {"context"},
	"internal/dml/dml.go":                      {"output"},
	"internal/dml/duplicate_rules.go":          {"b.m", "match"},
	"internal/gladecli/cli.go":                 {"info"},
	"internal/orgimport/sf.go":                 {"result", "object"},
	"internal/server/composite_handlers.go":    {"body"},
	"internal/storage/fixture.go":              {"fixtureObject"},
	"internal/storage/sqlite.go":               {"summary"},
	"internal/vm/platform_passive_members.go":  {"input"},
	"internal/vm/soql_runtime.go":              {"result"},
}

// TestObjectStateRecordsWritesUseFunnel fails when non-test code under
// internal/ or cmd/ writes ObjectState.Records outside the storage write
// funnel (PutRecord, DeleteRecord, SetRecords, ClearRecords, TouchRecords).
// A write that skips the funnel leaves the per-object write version stale, and
// any cache keyed on that version would then serve old rows.
//
// Contract limit: the token covers the set of rows and every row stored
// through the funnel. A row's own Fields map is shared with every copy of the
// row, so editing it in place (row := X.Records[id]; row.Fields[k] = v) is
// legal only when followed by PutRecord or TouchRecords. The scan cannot
// enforce that ordering, so it does not track row locals.
//
// The scan is syntactic. It flags index assignment, delete, clear, direct
// assignment of the field, and the same writes through a local variable
// that was assigned from a .Records selector.
func TestObjectStateRecordsWritesUseFunnel(t *testing.T) {
	root := canonicalRepoRoot
	var findings []string
	fset := token.NewFileSet()
	for _, top := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				switch entry.Name() {
				case "node_modules", "testdata", ".git":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if slices.Contains(recordWriteFunnelExempt, rel) {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return fmt.Errorf("parse %s: %w", rel, err)
			}
			for _, finding := range scanRecordWrites(fset, file) {
				if recordsOwnerAllowed(rel, finding.recv) {
					continue
				}
				findings = append(findings, fmt.Sprintf("%s:%d: %s", rel, finding.line, finding.what))
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(findings)
	if len(findings) > 0 {
		t.Fatalf("ObjectState.Records written outside the storage funnel; use PutRecord, DeleteRecord, SetRecords, ClearRecords or TouchRecords:\n%s", strings.Join(findings, "\n"))
	}
}

type recordWriteFinding struct {
	line int
	what string
	recv string
}

func recordsOwnerAllowed(rel, recv string) bool {
	return slices.Contains(recordsFieldOwners[rel], recv)
}

func exprText(fset *token.FileSet, expr ast.Expr) string {
	var b strings.Builder
	var walk func(ast.Expr)
	walk = func(e ast.Expr) {
		switch x := e.(type) {
		case *ast.Ident:
			b.WriteString(x.Name)
		case *ast.SelectorExpr:
			walk(x.X)
			b.WriteString("." + x.Sel.Name)
		case *ast.IndexExpr:
			walk(x.X)
			b.WriteString("[]")
		case *ast.StarExpr:
			b.WriteString("*")
			walk(x.X)
		case *ast.ParenExpr:
			walk(x.X)
		case *ast.CallExpr:
			walk(x.Fun)
			b.WriteString("()")
		default:
			b.WriteString("?")
		}
	}
	walk(expr)
	return b.String()
}

// recordsSelector returns the receiver text when expr is X.Records.
func recordsSelector(fset *token.FileSet, expr ast.Expr) (string, bool) {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			break
		}
		expr = paren.X
	}
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Records" {
		return "", false
	}
	return exprText(fset, sel.X), true
}

func scanRecordWrites(fset *token.FileSet, file *ast.File) []recordWriteFinding {
	var out []recordWriteFinding
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		out = append(out, scanRecordWritesIn(fset, fn.Body)...)
	}
	return out
}

// recordsReadOnlyCallees names functions and methods that take a Records map
// (or an alias of one) and only read it.
var recordsReadOnlyCallees = map[string]bool{
	"len": true, "cap": true, "range": true,
	"LookupRecordByID": true, "cloneRecordMap": true, "sortedStorageRecords": true,
	"sortedCustomDataRecords": true, "cartChildRecords": true, "canonicalSummaryParentIDs": true,
	"next": true, "revokeChangedUsers": true,
	"Sprintf": true, "Fprintf": true, "Printf": true, "Errorf": true, "Marshal": true,
}

func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// nestedArg reports a map argument that lives inside a stored row, such as
// X.Records[id].Fields in delete(X.Records[id].Fields, k).
func nestedArg(arg ast.Expr, nested func(ast.Expr) (string, bool)) (string, bool) {
	if _, ok := arg.(*ast.SelectorExpr); !ok {
		return "", false
	}
	return nested(arg)
}

func scanRecordWritesIn(fset *token.FileSet, body *ast.BlockStmt) []recordWriteFinding {
	var out []recordWriteFinding
	aliases := map[string]string{}
	line := func(n ast.Node) int { return fset.Position(n.Pos()).Line }
	var source func(expr ast.Expr) (string, bool)
	source = func(expr ast.Expr) (string, bool) {
		for {
			paren, ok := expr.(*ast.ParenExpr)
			if !ok {
				break
			}
			expr = paren.X
		}
		if recv, ok := recordsSelector(fset, expr); ok {
			return recv, true
		}
		if id, ok := expr.(*ast.Ident); ok {
			recv, ok := aliases[id.Name]
			return recv, ok
		}
		return "", false
	}
	// Collect aliases to a fixed point: := and = assignments, var specs, and
	// copies of an alias.
	for changed := true; changed; {
		changed = false
		bind := func(name ast.Expr, value ast.Expr) {
			id, ok := name.(*ast.Ident)
			if !ok || id.Name == "_" {
				return
			}
			if recv, ok := source(value); ok {
				if _, seen := aliases[id.Name]; !seen {
					aliases[id.Name] = recv
					changed = true
				}
			}
		}
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				if len(x.Lhs) == len(x.Rhs) {
					for i := range x.Rhs {
						bind(x.Lhs[i], x.Rhs[i])
					}
				}
			case *ast.ValueSpec:
				if len(x.Names) == len(x.Values) {
					for i := range x.Values {
						bind(x.Names[i], x.Values[i])
					}
				}
			}
			return true
		})
	}
	target := func(expr ast.Expr) (string, string, bool) {
		recv, ok := source(expr)
		if !ok {
			return "", "", false
		}
		if _, direct := recordsSelector(fset, expr); direct {
			return recv, "direct write", true
		}
		return recv, "write through alias", true
	}
	// nested reports a write whose target chain indexes into a stored row, as
	// in X.Records[id].Fields[name] = value. The row's maps are shared with
	// the stored record.
	nested := func(expr ast.Expr) (string, bool) {
		for {
			switch e := expr.(type) {
			case *ast.IndexExpr:
				if _, _, ok := target(e.X); ok {
					recv, _ := source(e.X)
					return recv, true
				}
				expr = e.X
			case *ast.SelectorExpr:
				expr = e.X
			case *ast.ParenExpr:
				expr = e.X
			default:
				return "", false
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range x.Lhs {
				if ix, ok := lhs.(*ast.IndexExpr); ok {
					if recv, what, ok := target(ix.X); ok {
						out = append(out, recordWriteFinding{line(n), what + ": index assignment", recv})
						continue
					}
				}
				if recv, ok := recordsSelector(fset, lhs); ok {
					out = append(out, recordWriteFinding{line(n), "direct write: field assignment", recv})
				} else if recv, ok := nested(lhs); ok {
					out = append(out, recordWriteFinding{line(n), "write into a stored row", recv})
				}
			}
		case *ast.IncDecStmt:
			if recv, ok := nested(x.X); ok {
				out = append(out, recordWriteFinding{line(n), "write into a stored row", recv})
			}
		case *ast.CallExpr:
			name := calleeName(x.Fun)
			for i, arg := range x.Args {
				recv, what, ok := target(arg)
				if !ok {
					if nestedRecv, isNested := nestedArg(arg, nested); isNested && (name == "delete" || name == "clear") && i == 0 {
						out = append(out, recordWriteFinding{line(n), "write into a stored row: " + name, nestedRecv})
					}
					continue
				}
				switch {
				case (name == "delete" || name == "clear") && i == 0:
					out = append(out, recordWriteFinding{line(n), what + ": " + name, recv})
				case recordsReadOnlyCallees[name]:
				default:
					out = append(out, recordWriteFinding{line(n), what + " passed to " + name + "; it may mutate the map", recv})
				}
			}
		}
		return true
	})
	return out
}

func TestScanRecordWritesFlagsDirectWrites(t *testing.T) {
	const src = `package p
func f(org *Org, object State) {
	object.Records[id] = rec
	delete(object.Records, id)
	clear(object.Records)
	object.Records = nil
	records := org.Objects["X"].Records
	records[id] = rec
	delete(records, id)
	maps.Copy(records, other)
	_ = object.Records[id]
	for range object.Records {
	}
	object.PutRecord(id, rec)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	got := scanRecordWrites(fset, file)
	var lines []int
	for _, finding := range got {
		lines = append(lines, finding.line)
	}
	want := []int{3, 4, 5, 6, 8, 9, 10}
	if !slices.Equal(lines, want) {
		t.Fatalf("flagged lines = %v, want %v", lines, want)
	}
}

func TestScanRecordWritesFlagsAliasAndNestedShapes(t *testing.T) {
	const src = `package p
func f(object State) {
	var rows = object.Records
	rows[id] = rec
	first := object.Records
	other := first
	delete(other, id)
	object.Records[id].Fields["Name"] = value
	mutate(object.Records)
	delete(object.Records[id].Fields, "Name")
	alias := object.Records
	helper(alias)
	_ = len(object.Records)
	_ = LookupRecordByID(object.Records, id)
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
	}
	var lines []int
	for _, finding := range scanRecordWrites(fset, file) {
		lines = append(lines, finding.line)
	}
	want := []int{4, 7, 8, 9, 10, 12}
	if !slices.Equal(lines, want) {
		t.Fatalf("flagged lines = %v, want %v", lines, want)
	}
}
