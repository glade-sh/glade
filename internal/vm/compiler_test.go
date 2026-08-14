package vm

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
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
