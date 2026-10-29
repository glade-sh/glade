package sema

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// A source inner type sharing a catalog name must not capture a call on an
// unrelated constructed receiver. This is a regression control, not a native row.
func TestInnerReceiverChainedConstructorRecovery(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"Fluent.cls": `public class Fluent {
            public static Builder start() { return new Builder(); }
            public class Builder {
                public Builder step() { return this; }
                public Builder accept(String value) { return this; }
                public Boolean finish() { return true; }
            }
        }`,
		"ResultFluent.cls": `public class ResultFluent {
            public static SaveResult start() { return new SaveResult(); }
            public class SaveResult {
                public SaveResult accept(String value) { return this; }
                public Boolean finish() { return true; }
            }
        }`,
		"FieldFluent.cls": `public class FieldFluent {
            public static DescribeFieldResult start() { return new DescribeFieldResult(); }
            public class DescribeFieldResult {
                public DescribeFieldResult accept(String value) { return this; }
                public Boolean finish() { return true; }
            }
        }`,
		"FirstService.cls":  `public class FirstService { public FirstService step() { return this; } public String lookup(String key) { return key; } }`,
		"SecondService.cls": `public class SecondService { public String search(String key) { return key; } }`,
		"ThirdService.cls":  `public class ThirdService { public String fetch(String key) { return key; } }`,
		"UseServices.cls": `@IsTest private class UseServices {
            @IsTest static void exercise() {
                Boolean first = Fluent.start().step().accept(
                    new FirstService().lookup('one')
                ).finish();
                Boolean second = Fluent.start().step().accept(
                    new SecondService().search('two')
                ).finish();
                Boolean result = ResultFluent.start().accept(
                    new FirstService().lookup('four')
                ).finish();
                Boolean field = FieldFluent.start().accept(
                    new SecondService().search('five')
                ).finish();
                Boolean chained = ResultFluent.start().accept(
                    new FirstService().step().lookup('six')
                ).finish();
                Boolean third = Fluent.start().step().accept(
                    new ThirdService().fetch('three')
                ).finish();
            }
        }`,
	}
	var names []string
	for name, source := range files {
		writeSemaFile(t, filepath.Join(root, name), source)
		names = append(names, name)
	}
	sort.Strings(names)
	paths := make([]string, 0, len(names))
	for _, name := range names {
		paths = append(paths, filepath.Join(root, name))
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
			result := Analyze(index)
			if result.HasErrors() {
				t.Fatalf("unexpected analysis diagnostics: %#v", result.Diagnostics)
			}
			a := NewAnalyzer()
			a.prepareAnalysisContext(index, AnalyzeOptions{})
			a.prepareAnalysisModel(index)
			model := buildSemaTypeMemberView(index)
			checked := false
			for _, typ := range index.Types {
				if typ.Name != "UseServices" {
					continue
				}
				source, err := os.ReadFile(typ.File)
				if err != nil {
					t.Fatal(err)
				}
				typeModel := semaModelWithCurrentType(model, typ)
				for _, member := range typ.Members {
					if member.Kind != apexast.DeclarationMethod || member.Name != "exercise" {
						continue
					}
					checked = true
					body, offset, ok := semaBodyFromRange(string(source), member.BodyRange)
					if !ok {
						t.Fatal("missing test body")
					}
					member = semaNormalizeMemberTypes(typeModel, typ.Name, semaCloneMemberSymbol(member))
					base := semaBodyBaseScope(typ, member, typeModel)
					scan := newSemaBodyExpressionScan(body)
					scopes, diagnostics := a.collectBodyScopesWithLocalDeclMatches(typ, member, body, offset, string(source), base, typeModel, scan.localDeclMatches)
					diagnostics = append(diagnostics, a.checkBodyCalls(typ, member, body, offset, string(source), scopes, typeModel)...)
					if len(diagnostics) != 0 {
						t.Fatalf("unexpected text route diagnostics: %#v", diagnostics)
					}
				}
			}
			if !checked {
				t.Fatal("test method not indexed")
			}
		})
	}
}
