package sema

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestSOQLBindLexicalCaseDeclarations(t *testing.T) {
	// DML T005/T006 bind a Case local declared earlier in the same try block,
	// on both routes at APIs 62/67/68. Account R003/R004 are unchanged neighbors.
	for _, wrapper := range []string{"%s", "@IsTest private class BindProbe { @IsTest static void observe(){%s} }"} {
		for _, typ := range []string{"Case", "Account"} {
			body := `try{Database.DMLOptions options=new Database.DMLOptions(); ` + typ + ` x=new ` + typ + `(); Object r=[SELECT Id FROM ` + typ + ` WHERE Id=:x.Id];}catch(Exception e){}`
			source := fmt.Sprintf(wrapper, body)
			offset := strings.Index(source, ":x.Id")
			bindings := newSemaBindingResolver(source, newSemaCodeSpans(source)).bindingsAt(offset)
			if got := bindings["x"]; got != typ {
				t.Fatalf("%s binding=%q, want %s; all=%#v", wrapper, got, typ, bindings)
			}
			if diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("BindProbe.cls", source); hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
				t.Fatalf("%s bind rejected: %#v", typ, diagnostics)
			}
		}
	}
}

func TestSOQLBindLexicalNestedBlocks(t *testing.T) {
	// Scope-only checks avoid project builds and runtime matrix expansion.
	// Before-declaration and closed-block controls retain LX054/056 rejection.
	const query = `System.debug([SELECT Id FROM Account WHERE Id=:x.Id]);`
	for _, block := range []struct{ name, open, close string }{
		{"try", "try{", "}catch(Exception e){}"},
		{"catch", "try{}catch(Exception e){", "}"},
		{"finally", "try{}finally{", "}"},
		{"for", "for(Integer i=0;i<1;i++){", "}"},
		{"foreach", "for(Account item:new List<Account>()){", "}"},
		{"while", "while(false){", "}"},
		{"if", "if(true){", "}"},
		{"else", "if(false){}else{", "}"},
		{"switchWhen", "switch on 1{when 1{", "}}"},
		{"nested", "{", "}"},
	} {
		t.Run(block.name, func(t *testing.T) {
			for _, wrapper := range []string{"%s", "public class BindProbe { public static void observe(){%s} }"} {
				for _, prefix := range []string{"", "Integer marker=0;"} {
					body := block.open + prefix + query + `Account x=new Account();` + query + `{` + query + `}` + block.close + query
					source := fmt.Sprintf(wrapper, body)
					resolver := newSemaBindingResolver(source, newSemaCodeSpans(source))
					search := 0
					for i := 0; i < 4; i++ {
						index := strings.Index(source[search:], ":x.Id")
						if index < 0 {
							t.Fatal("missing scope query")
						}
						offset := search + index
						search = offset + len(":x.Id")
						want := ""
						if i == 1 || i == 2 {
							want = "Account"
						}
						if got := resolver.bindingsAt(offset)["x"]; got != want {
							t.Fatalf("%s prefix=%q query=%d: binding=%q, want %q", wrapper, prefix, i, got, want)
						}
					}
					// Put the declaration first, with a later bind in a nested block.
					body = block.open + prefix + `Account x=new Account();` + query + `{` + query + `}` + block.close
					source = fmt.Sprintf(wrapper, body)
					resolver = newSemaBindingResolver(source, newSemaCodeSpans(source))
					for _, offset := range []int{strings.Index(source, ":x.Id"), strings.LastIndex(source, ":x.Id")} {
						if got := resolver.bindingsAt(offset)["x"]; got != "Account" {
							t.Fatalf("first declaration not visible: %s prefix=%q binding=%q", wrapper, prefix, got)
						}
					}
				}
			}
		})
	}
}
