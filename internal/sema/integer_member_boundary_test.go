package sema

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// C071/C072 have identical native anonymous and named compiler rejections at
// all five APIs. Renamed members and source-owned receivers below are local regression
// controls for the shared type-based rule, not additional native observations.
func TestIntegerMemberBoundaryDiagnostics(t *testing.T) {
	cases := []struct {
		name, body, want, sourceType string
	}{
		{"captured method", "Integer x=1; x.noSuchSyntaxMethod();", "Method does not exist or incorrect signature: void noSuchSyntaxMethod() from the type Integer", ""},
		{"captured field", "Integer x=1; x.noSuchSyntaxField=2;", "Variable does not exist: noSuchSyntaxField", ""},
		{"renamed method", "Integer value=1; value.unavailableMember();", "Method does not exist or incorrect signature: void unavailableMember() from the type Integer", ""},
		{"renamed field", "Integer value=1; value.unavailableMember=2;", "Variable does not exist: unavailableMember", ""},
		{"known members", "Integer x=1; String formatted=x.format(); Integer parsed=Integer.valueOf('2');", "", ""},
		// Local regression controls: these are lowered operator calls, not
		// additional native observations or new increment-shape support.
		{"postfix increment", "Integer value=1; value++;", "", ""},
		{"prefix increment", "Integer value=1; ++value;", "", ""},
		{"postfix decrement", "Integer value=1; value--;", "", ""},
		{"prefix decrement", "Integer value=1; --value;", "", ""},
		// Return expressions retain synthetic callees for all four operators.
		{"return postfix increment", "Integer observed=MemberReceiver.next();", "", `public class MemberReceiver { public static Integer next() { Integer value=1; return value++; } }`},
		{"return prefix increment", "Integer observed=MemberReceiver.next();", "", `public class MemberReceiver { public static Integer next() { Integer value=1; return ++value; } }`},
		{"return postfix decrement", "Integer observed=MemberReceiver.next();", "", `public class MemberReceiver { public static Integer next() { Integer value=1; return value--; } }`},
		{"return prefix decrement", "Integer observed=MemberReceiver.next();", "", `public class MemberReceiver { public static Integer next() { Integer value=1; return --value; } }`},
		{"operator then missing method", "Integer value=1; value++; value.noSuchSyntaxMethod();", "Method does not exist or incorrect signature: void noSuchSyntaxMethod() from the type Integer", ""},
		{"source-owned receiver", "MemberReceiver x=new MemberReceiver(); x.noSuchSyntaxMethod(); x.noSuchSyntaxField=2;", "", `public class MemberReceiver { public Integer noSuchSyntaxField; public void noSuchSyntaxMethod() {} }`},
	}
	for _, api := range []string{"62.0", "65.0", "66.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					build := func(named bool) typesys.Index {
						root := t.TempDir()
						var paths []string
						write := func(name, source string) {
							path := filepath.Join(root, name+".cls")
							writeSemaFile(t, path, source)
							writeSemaFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
							paths = append(paths, path)
						}
						if tc.sourceType != "" {
							write("MemberReceiver", tc.sourceType)
						}
						if named {
							write("IntegerBoundary", "public class IntegerBoundary { public static void observe(){\n"+tc.body+"\n} }")
						}
						index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
						if index.HasErrors() {
							t.Fatalf("declaration parsing: %v", index.Diagnostics)
						}
						return index
					}
					check := func(t *testing.T, result Result) {
						t.Helper()
						var messages []string
						for _, d := range result.Diagnostics {
							if d.Severity == diagnostic.Error {
								message := d.NativeMessage
								if message == "" {
									message = d.Message
								}
								messages = append(messages, message)
							}
						}
						if tc.want == "" {
							if len(messages) != 0 {
								t.Fatalf("valid members rejected: %v", messages)
							}
						} else if len(messages) != 1 || messages[0] != tc.want {
							t.Fatalf("expected one diagnostic <%s>; got %v", tc.want, messages)
						}
					}
					t.Run("anonymous", func(t *testing.T) { check(t, AnalyzeAnonymous(build(false), tc.body, api)) })
					t.Run("named", func(t *testing.T) { check(t, Analyze(build(true))) })
				})
			}
		})
	}
}
