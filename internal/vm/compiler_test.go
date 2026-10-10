package vm

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/soql"
)

func TestCompileDMLAccessModesPreservePrefixAndSuffixSyntax(t *testing.T) {
	for _, operation := range []struct {
		name       string
		defaultSrc string
		userSrc    string
		systemSrc  string
	}{
		{name: "insert", defaultSrc: "insert record;", userSrc: "insert as user record;", systemSrc: "insert as system record;"},
		{name: "update", defaultSrc: "update record;", userSrc: "update record as user;", systemSrc: "update record as system;"},
		{name: "upsert", defaultSrc: "upsert record External_Id__c;", userSrc: "upsert record External_Id__c as user;", systemSrc: "upsert as system record External_Id__c;"},
		{name: "delete", defaultSrc: "delete record;", userSrc: "delete as user record;", systemSrc: "delete record as system;"},
		{name: "undelete", defaultSrc: "undelete record;", userSrc: "undelete as user record;", systemSrc: "undelete record as system;"},
		{name: "merge", defaultSrc: "merge master duplicate;", userSrc: "merge master duplicate as user;", systemSrc: "merge as system master duplicate;"},
	} {
		for _, test := range []struct {
			name string
			src  string
			mode ir.DMLMode
		}{
			{name: "default", src: operation.defaultSrc, mode: ir.DMLModeDefault},
			{name: "user", src: operation.userSrc, mode: ir.DMLModeUser},
			{name: "system", src: operation.systemSrc, mode: ir.DMLModeSystem},
		} {
			t.Run(operation.name+"/"+test.name, func(t *testing.T) {
				program, err := CompileAnonymous(test.src)
				if err != nil {
					t.Fatalf("CompileAnonymous(%q): %v", test.src, err)
				}
				if len(program.Instructions) != 1 || program.Instructions[0].DMLMode != test.mode {
					t.Fatalf("DML mode for %q = %#v, want %d", test.src, program.Instructions, test.mode)
				}
			})
		}
	}
}

func TestCompileDMLAccessModesSurviveControlFlow(t *testing.T) {
	program, err := CompileAnonymous("if (true) { update as user record; } else { delete as system record; }")
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Instructions) != 1 {
		t.Fatalf("instructions = %#v", program.Instructions)
	}
	if got := program.Instructions[0].Then[0].DMLMode; got != ir.DMLModeUser {
		t.Fatalf("then DML mode = %d, want user", got)
	}
	if got := program.Instructions[0].Else[0].DMLMode; got != ir.DMLModeSystem {
		t.Fatalf("else DML mode = %d, want system", got)
	}
}

func TestCompileDMLAccessModesRejectDuplicatesAndSurviveJSON(t *testing.T) {
	if _, err := CompileAnonymous("insert as user record as system;"); err == nil {
		t.Fatal("duplicate DML access modes were accepted")
	}
	program, err := CompileAnonymous("undelete as system record;")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(program)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip ir.Program
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if got := roundTrip.Instructions[0].DMLMode; got != ir.DMLModeSystem {
		t.Fatalf("JSON DML mode = %d, want %d", got, ir.DMLModeSystem)
	}
}

func TestCompileMultilineStringLiteral(t *testing.T) {
	for _, test := range []struct {
		name    string
		src     string
		want    string
		wantErr bool
	}{
		{name: "opening newline is trimmed", src: "String value = '''\nhello\nworld\n''';", want: "hello\nworld\n"},
		{name: "CRLF is normalized", src: "String value = '''\r\nhello\r\nworld\r\n''';", want: "hello\nworld\n"},
		{name: "same line opening is rejected", src: "String value = '''hello''';", wantErr: true},
		{name: "empty opening is rejected", src: "String value = '''''';", wantErr: true},
		{name: "empty after opening newline", src: "String value = '''\n''';", want: ""},
		{name: "quotes and backslash are preserved", src: "String value = '''\n'quote' \\n\n''';", want: "'quote' \\n\n"},
		{name: "quote runs are preserved", src: "String value = '''\none ' quote\ntwo '' quotes\n''';", want: "one ' quote\ntwo '' quotes\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymous(test.src)
			if test.wantErr {
				if err == nil {
					t.Fatalf("CompileAnonymous(%q) succeeded, want error", test.src)
				}
				return
			}
			if err != nil {
				t.Fatalf("CompileAnonymous(%q): %v", test.src, err)
			}
			literal, err := parseLiteral(firstLiteralValue(program))
			if err != nil {
				t.Fatal(err)
			}
			if literal.Text != test.want {
				t.Fatalf("literal = %q, want %q", literal.Text, test.want)
			}
		})
	}
}

func firstLiteralValue(program ir.Program) string {
	for _, instruction := range program.Instructions {
		if instruction.Expr.Kind == ir.ExprLiteral {
			return instruction.Expr.Value
		}
		for _, arg := range instruction.Expr.Args {
			if arg.Kind == ir.ExprLiteral {
				return arg.Value
			}
		}
	}
	return ""
}

func TestCompileAPI67InlineSOQLPreservesBackslashEscapes(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
String apexPath = 'C:\\Trail';
List<Account> rows = [SELECT Id FROM Account WHERE Name = 'C:\\Trail'];
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Instructions) != 2 {
		t.Fatalf("instructions = %#v, want ordinary string and inline SOQL", program.Instructions)
	}
	apexPath, err := parseLiteral(firstLiteralValue(program))
	if err != nil {
		t.Fatalf("parse ordinary Apex string literal: %v", err)
	}
	if got, want := apexPath.Text, `C:\Trail`; got != want {
		t.Errorf("ordinary Apex string = %q, want %q", got, want)
	}
	if got, want := program.Instructions[1].Expr.Value, `SELECT Id FROM Account WHERE Name = 'C:\\Trail'`; got != want {
		t.Errorf("inline SOQL = %q, want %q", got, want)
	}
}

func TestCompileAPI67InlineSOQLNormalizesEscapedQuote(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`
List<Account> rows = [SELECT Id FROM Account WHERE Name = 'Bob\'s Shop'];
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if len(program.Instructions) != 1 {
		t.Fatalf("instructions = %#v, want one inline SOQL declaration", program.Instructions)
	}
	if got, want := program.Instructions[0].Expr.Value, `SELECT Id FROM Account WHERE Name = 'Bob''s Shop'`; got != want {
		t.Fatalf("inline SOQL = %q, want normalized quote text %q", got, want)
	}
}

func TestCompileAPI67InlineSOQLNormalizesUnicodeEscapes(t *testing.T) {
	for _, test := range []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "surrogate pair",
			source: `List<Account> rows = [SELECT Id FROM Account WHERE Name = '\uD83D\uDE00'];`,
			want:   `SELECT Id FROM Account WHERE Name = '😀'`,
		},
		{
			name:   "BMP character",
			source: `List<Account> rows = [SELECT Id FROM Account WHERE Name = '\u0041'];`,
			want:   `SELECT Id FROM Account WHERE Name = 'A'`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(test.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if len(program.Instructions) != 1 {
				t.Fatalf("instructions = %#v, want one inline SOQL declaration", program.Instructions)
			}
			if got := program.Instructions[0].Expr.Value; got != test.want {
				t.Fatalf("inline SOQL = %q, want decoded Unicode %q", got, test.want)
			}
		})
	}
}

func TestCompileAPI67InlineSOQLPreservesAdjacentBackslashEscapes(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name:   "Unicode backslash before Apex escaped backslash",
			source: `List<Account> rows = [SELECT Id FROM Account WHERE Name = '😀\u005C\\'];`,
		},
		{
			name:   "Apex escaped backslash before Unicode backslash",
			source: `List<Account> rows = [SELECT Id FROM Account WHERE Name = '😀\\\u005C'];`,
		},
	}
	wantQuery := `SELECT Id FROM Account WHERE Name = '😀\\\\'`
	wantValue := "😀\\\\"
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(test.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			if len(program.Instructions) != 1 {
				t.Fatalf("instructions = %#v, want one inline SOQL declaration", program.Instructions)
			}
			gotQuery := program.Instructions[0].Expr.Value
			if gotQuery != wantQuery {
				t.Fatalf("inline SOQL = %q, want %q", gotQuery, wantQuery)
			}
			query, err := soql.Parse(gotQuery)
			if err != nil {
				t.Fatalf("parse reconstructed SOQL: %v", err)
			}
			if query.Where == nil || query.Where.Value.String != wantValue {
				t.Fatalf("parsed string = %#v, want %q", query.Where, wantValue)
			}
		})
	}
}

// Only prefix candidates lower as expressions, and only when semantic analysis
// approved their offset; candidates mode lowers all of them for that analysis.
// Every other case fails exactly as the name path does.
func TestCompilePrefixStatementsLowerOnlyApprovedCandidates(t *testing.T) {
	const locals = "Map<String,Account> m = new Map<String,Account>();\nString key = 'one';\nAccount acc = new Account();\nList<Long> l = new List<Long>{2};\n"
	for statement, candidate := range map[string]bool{
		"++l[0];":                                true,
		"--l[0].AnnualRevenue;":                  true,
		"++m.get('one').AnnualRevenue;":          true,
		"--/*c*/m.get('one').AnnualRevenue;":     true,
		"++m.get(key).AnnualRevenue;":            true,
		"--m.get(acc.Id).AnnualRevenue;":         true,
		"++m.get(acc.Owner.Name).AnnualRevenue;": false,
		"++m.get(key).Owner.Name;":               false,
		"++m.get(key);":                          false,
		"++l[0][0];":                             false,
	} {
		source := locals + statement
		_, namePath := CompileAnonymous(source)
		if namePath == nil {
			t.Fatalf("%s: the name path lowered it", statement)
		}
		at := map[int]bool{len(locals): true}
		for name, options := range map[string]CompileOptions{
			"candidates": {PrefixStatementCandidates: true},
			"approved":   {ApprovedPrefixStatements: at},
		} {
			_, err := CompileAnonymousWithOptions(source, options)
			if candidate && err != nil {
				t.Fatalf("%s %s: %v", statement, name, err)
			}
			if !candidate && (err == nil || err.Error() != namePath.Error()) {
				t.Fatalf("%s %s: got %v, want the name-path error %v", statement, name, err, namePath)
			}
		}
		// A candidate at an offset nobody approved keeps the name path.
		_, err := CompileAnonymousWithOptions(source, CompileOptions{ApprovedPrefixStatements: map[int]bool{len(locals) + 1: true}})
		if err == nil || err.Error() != namePath.Error() {
			t.Fatalf("%s unapproved: got %v, want the name-path error %v", statement, err, namePath)
		}
	}
}
