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
	"internal/dbmanager/lookup.go":            {"out"},
	"internal/dml/automation.go":              {"context"},
	"internal/dml/dml.go":                     {"output"},
	"internal/dml/duplicate_rules.go":         {"b.m", "match"},
	"internal/gladecli/cli.go":                {"info"},
	"internal/orgimport/sf.go":                {"result", "object"},
	"internal/server/composite_handlers.go":   {"body"},
	"internal/storage/fixture.go":             {"fixtureObject"},
	"internal/storage/sqlite.go":              {"summary"},
	"internal/vm/platform_passive_members.go": {"input"},
	"internal/vm/soql_runtime.go":             {"result"},
}

// TestObjectStateRecordsWritesUseFunnel fails when non-test code under
// internal/ or cmd/ writes ObjectState.Records outside the storage write
// funnel (PutRecord, DeleteRecord, SetRecords, ClearRecords, TouchRecords).
// A write that skips the funnel leaves the per-object write version stale, and
// any cache keyed on that version would then serve old rows.
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

func scanRecordWritesIn(fset *token.FileSet, body *ast.BlockStmt) []recordWriteFinding {
	var out []recordWriteFinding
	aliases := map[string]string{}
	line := func(n ast.Node) int { return fset.Position(n.Pos()).Line }
	// target reports whether expr is a Records selector or an alias of one.
	target := func(expr ast.Expr) (string, string, bool) {
		if recv, ok := recordsSelector(fset, expr); ok {
			return recv, "direct write", true
		}
		if id, ok := expr.(*ast.Ident); ok {
			if recv, ok := aliases[id.Name]; ok {
				return recv, "write through alias " + id.Name, true
			}
		}
		return "", "", false
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for i, rhs := range x.Rhs {
				if recv, ok := recordsSelector(fset, rhs); ok && len(x.Lhs) == len(x.Rhs) {
					if id, ok := x.Lhs[i].(*ast.Ident); ok && id.Name != "_" {
						aliases[id.Name] = recv
					}
				}
			}
			for _, lhs := range x.Lhs {
				if ix, ok := lhs.(*ast.IndexExpr); ok {
					if recv, what, ok := target(ix.X); ok {
						out = append(out, recordWriteFinding{line(n), what + ": index assignment", recv})
					}
				} else if recv, ok := recordsSelector(fset, lhs); ok {
					out = append(out, recordWriteFinding{line(n), "direct write: field assignment", recv})
				}
			}
		case *ast.IncDecStmt:
			if ix, ok := x.X.(*ast.IndexExpr); ok {
				if recv, what, ok := target(ix.X); ok {
					out = append(out, recordWriteFinding{line(n), what + ": index update", recv})
				}
			}
		case *ast.CallExpr:
			if id, ok := x.Fun.(*ast.Ident); ok && (id.Name == "delete" || id.Name == "clear") && len(x.Args) > 0 {
				if recv, what, ok := target(x.Args[0]); ok {
					out = append(out, recordWriteFinding{line(n), what + ": " + id.Name, recv})
				}
			}
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok && len(x.Args) > 0 {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "maps" && (sel.Sel.Name == "Copy" || sel.Sel.Name == "DeleteFunc") {
					if recv, what, ok := target(x.Args[0]); ok {
						out = append(out, recordWriteFinding{line(n), what + ": maps." + sel.Sel.Name, recv})
					}
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
