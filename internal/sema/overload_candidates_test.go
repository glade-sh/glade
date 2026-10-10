package sema

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeOverloadCandidates(t *testing.T) {
	t.Parallel()
	for _, version := range []string{"61.0", "62.0", "67.0"} {
		for _, tc := range []struct {
			name            string
			files           map[string]string
			ambiguous       bool
			missingContract bool
		}{
			{
				// Platform dispatch R115 compiles execute(null,null) at both captured APIs.
				name: "batch interface implementation is one overload",
				files: map[string]string{
					"Widget.cls": `public class Widget implements Database.Batchable<Integer>, Schedulable {
  public Iterable<Integer> start(Database.BatchableContext context) { return new List<Integer>(); }
  public void execute(Database.BatchableContext context, List<Integer> records) {}
  public void execute(SchedulableContext context) {}
  public void finish(Database.BatchableContext context) {}
}`,
					"RecordBatch.cls": `public class RecordBatch implements Database.Batchable<SObject> {
  public Database.QueryLocator start(Database.BatchableContext context) { return null; }
  public void execute(Database.BatchableContext context, List<SObject> records) {}
  public void finish(Database.BatchableContext context) {}
}`,
					"Probe.cls": `public class Probe { public void run() {
  Widget value = new Widget(); value.execute(null, null);
  Database.Batchable<Integer> contract = value; contract.execute(null, null);
  contract.execute(null, new List<Integer>{1});
  RecordBatch other = new RecordBatch(); other.execute(null, null);
  Database.Batchable<SObject> second = other; second.execute(null, new List<SObject>());
  contract.execute(null, new List<Integer>{2});
} }`,
				},
			},
			{
				name: "batch SObject implementation is one overload",
				files: map[string]string{
					"Widget.cls": `public class Widget implements Database.Batchable<SObject> {
  public Database.QueryLocator start(Database.BatchableContext context) { return null; }
  public void execute(Database.BatchableContext context, List<SObject> records) {}
  public void finish(Database.BatchableContext context) {}
  public void run() { execute(null, null); }
}`,
				},
			},
			{
				name: "ordinary interface implementation is one overload",
				files: map[string]string{
					"Contract.cls": `public interface Contract { void pick(String value); }`,
					"Widget.cls": `public class Widget implements Contract {
  public void pick(String value) {} public void run() { pick(null); }
}`,
				},
			},
			{
				name:            "qualified interface rejection twin",
				ambiguous:       true,
				missingContract: true,
				files: map[string]string{
					"Left.cls":     `public class Left { public class Token {} }`,
					"Right.cls":    `public class Right { public class Token {} }`,
					"Contract.cls": `public interface Contract { void pick(Left.Token value); }`,
					"Widget.cls": `public class Widget implements Contract {
  public void pick(Right.Token value) {} public void run() { pick(null); }
}`,
				},
			},
			{
				name: "qualified interface positive twin",
				files: map[string]string{
					"Left.cls":     `public class Left { public class Token {} }`,
					"Contract.cls": `public interface Contract { void pick(Left.Token value); }`,
					"Widget.cls": `public class Widget implements Contract {
  public void pick(Left.Token value) {} public void run() { pick(null); }
}`,
				},
			},
			{
				name:            "qualified generic interface rejection twin",
				missingContract: true,
				files: map[string]string{
					"Left.cls":     `public class Left { public class Token {} }`,
					"Right.cls":    `public class Right { public class Token {} }`,
					"Contract.cls": `public interface Contract { void pick(List<Left.Token> value); }`,
					"Widget.cls": `public class Widget implements Contract {
  public void pick(List<Right.Token> value) {}
  public void run() { pick(null); }
}`,
				},
			},
			{
				name: "qualified generic interface positive twin",
				files: map[string]string{
					"Left.cls":     `public class Left { public class Token {} }`,
					"Contract.cls": `public interface Contract { void pick(List<Left.Token> value); }`,
					"Widget.cls": `public class Widget implements Contract {
  public void pick(List<Left.Token> value) {}
  public void pick(String label,Integer count) {}
  public void run() { pick(null); }
}`,
				},
			},
			{
				// Type-system R167: genuine String/Object null ambiguity stays rejected.
				name:      "distinct local static overloads remain ambiguous",
				ambiguous: true,
				files: map[string]string{
					"Widget.cls": `public class Widget {
  public static String pick(String value) { return 'string'; }
  public static Object pick(Object value) { return value; }
  public void run() { Widget.pick(null); }
}`,
				},
			},
			{
				// Type-system C010: crossed parameter lists are genuine overloads.
				name:      "crossed null arguments remain ambiguous",
				ambiguous: true,
				files: map[string]string{
					"Widget.cls": `public class Widget {
  public String pick(String left, Object right) { return 'left'; }
  public Object pick(Object left, String right) { return left; }
  public void run() { pick(null, null); }
}`,
				},
			},
			{
				// Type-system C009: typed arguments do not resolve a crossed tie.
				name:      "crossed typed arguments remain ambiguous",
				ambiguous: true,
				files: map[string]string{
					"Widget.cls": `public class Widget {
  public String pick(String left, Object right) { return 'left'; }
  public Object pick(Object left, String right) { return left; }
  public void run() { pick('x', 'y'); }
}`,
				},
			},
		} {
			t.Run(version+"/"+tc.name, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, tc.files, version)
				ambiguities := 0
				missingContracts := 0
				for _, d := range result.Diagnostics {
					if d.Code == "GLADESEMA022" && strings.Contains(d.Message, "ambiguous") {
						ambiguities++
					} else if d.Code == "GLADESEMA017" && tc.missingContract {
						missingContracts++
					} else {
						t.Errorf("unexpected diagnostic: %#v", d)
					}
				}
				want := 0
				if tc.ambiguous {
					want = 1
				}
				if ambiguities != want {
					t.Fatalf("ambiguities = %d, want %d: %#v", ambiguities, want, result.Diagnostics)
				}
				if tc.missingContract && missingContracts != 1 {
					t.Fatalf("qualified lookalike implemented the contract: %#v", result.Diagnostics)
				}
			})
		}
	}
}

func TestAnalyzeNarrowedBatchableSchedulableExecuteNullCall(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, call, wantCode string }{
			{"null null compiles", "job.execute(null, null);", ""},
			{"wrong scope rejection twin", "job.execute(null, 'invalid');", "GLADESEMA009"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				job := filepath.Join(root, "Job.cls")
				probe := filepath.Join(root, "Probe.cls")
				writeSemaFile(t, job, `public class Job implements Database.Batchable<SObject>, Schedulable {
  public Database.QueryLocator start(Database.BatchableContext ctx) { return null; }
  public void execute(Database.BatchableContext ctx, List<Widget__c> scope) {}
  public void execute(SchedulableContext ctx) {}
  public void finish(Database.BatchableContext ctx) {}
}`)
				writeSemaFile(t, probe, `@IsTest private class Probe {
  @IsTest static void observed() { Job job = new Job(); `+tc.call+` }
}`)
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{job, probe}},
					schema.Schema{Objects: []schema.Object{{Name: "Widget__c"}}})
				result := Analyze(index)
				if tc.wantCode == "" && result.HasErrors() {
					t.Fatalf("narrowed Batchable implementation competed with its requirement: %#v", result.Diagnostics)
				}
				if tc.wantCode != "" && (len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != tc.wantCode) {
					t.Fatalf("wrong scope diagnostics = %#v, want %s", result.Diagnostics, tc.wantCode)
				}
			})
		}
	}
}

func TestResolveNarrowedBatchableKeepsQualifiedApexScopeRejectionTwin(t *testing.T) {
	for _, tc := range []struct {
		scope string
		want  int
	}{
		{"List<Account>", 1},
		{"List<Right.Account>", 2},
	} {
		t.Run(tc.scope, func(t *testing.T) {
			index := typesys.Index{Objects: []schema.Object{{Name: "Account"}}, Types: []typesys.TypeSymbol{
				{Kind: apexast.DeclarationClass, Name: "Right.Account"},
				{Kind: apexast.DeclarationClass, Name: "Job", Interfaces: []string{"Database.Batchable<SObject>"}, Members: []typesys.MemberSymbol{
					{Kind: apexast.DeclarationMethod, Name: "execute", Type: "void", Modifiers: []string{"public"}, Parameters: []apexast.Parameter{
						{Name: "ctx", Type: "Database.BatchableContext"}, {Name: "scope", Type: tc.scope},
					}},
				}},
			}}
			candidates := resolveMemberMethods(buildSemaTypeMemberView(index), "Job", "execute")
			if len(candidates) != tc.want {
				t.Fatalf("candidates = %#v, want %d", candidates, tc.want)
			}
		})
	}
}

// Captured C002/C006/R009-R011 require retaining inherited candidates.
// Static preference is applied after argument applicability, not during lookup.
func TestResolveStaticCandidatesRetainsAncestorOverload(t *testing.T) {
	for _, visibility := range []string{"public", "private", "protected"} {
		for _, deep := range []bool{false, true} {
			t.Run(visibility+fmt.Sprint(deep), func(t *testing.T) {
				method := func(param string, access string) typesys.MemberSymbol {
					return typesys.MemberSymbol{Kind: apexast.DeclarationMethod, Name: "pick", Type: "String", Modifiers: []string{access, "static"}, Parameters: []apexast.Parameter{{Name: "value", Type: param}}}
				}
				parent := "Base"
				if deep {
					parent = "Middle"
				}
				index := typesys.Index{Types: []typesys.TypeSymbol{
					{Kind: apexast.DeclarationClass, Name: "Base", Members: []typesys.MemberSymbol{method("Integer", "public")}},
					{Kind: apexast.DeclarationClass, Name: "Middle", SuperClass: "Base"},
					{Kind: apexast.DeclarationClass, Name: "Child", SuperClass: parent, Members: []typesys.MemberSymbol{method("String", visibility)}},
				}}
				candidates := resolveMemberMethods(buildSemaTypeMemberView(index), "Child", "pick")
				keys := map[string]bool{}
				for _, candidate := range candidates {
					keys[methodSignatureKey(candidate.member)] = true
				}
				if len(candidates) != 2 || !keys["pick/integer"] || !keys["pick/string"] {
					t.Fatalf("ancestor candidate suppressed: %#v", candidates)
				}
			})
		}
	}
}

func TestResolveQualifiedInterfaceCandidatesRemainDistinct(t *testing.T) {
	for _, pair := range [][2]string{{"Left.Token", "Right.Token"}, {"one.Token", "two.Token"}, {"List<Left.Token>", "List<Right.Token>"}, {"List<one.Token>", "List<two.Token>"}} {
		t.Run(pair[0], func(t *testing.T) {
			method := func(param string) typesys.MemberSymbol {
				return typesys.MemberSymbol{Kind: apexast.DeclarationMethod, Name: "pick", Type: "void", Modifiers: []string{"public"}, Parameters: []apexast.Parameter{{Name: "value", Type: param}}}
			}
			index := typesys.Index{Types: []typesys.TypeSymbol{
				{Kind: apexast.DeclarationInterface, Name: "Contract", Members: []typesys.MemberSymbol{method(pair[0])}},
				{Kind: apexast.DeclarationClass, Name: "Widget", Interfaces: []string{"Contract"}, Members: []typesys.MemberSymbol{method(pair[1])}},
			}}
			candidates := resolveMemberMethods(buildSemaTypeMemberView(index), "Widget", "pick")
			if len(candidates) != 2 {
				t.Fatalf("qualified requirement pruned: %#v", candidates)
			}
			if methodSignatureKey(candidates[0].member) == methodSignatureKey(candidates[1].member) {
				t.Fatal("qualified signatures conflated")
			}
		})
	}
}

// Captured type system controls: local null selects the derived static declaration;
// typed ancestors remain reachable and inaccessible local choices still reject.
func TestAnalyzeCapturedStaticOverloadPrecedence(t *testing.T) {
	for _, api := range []string{"61.0", "62.0", "67.0"} {
		for _, tc := range []struct{ name, receiver, call, code string }{
			{"public null", "Child", "null", ""},
			{"deep null", "DeepChild", "null", ""},
			{"public ancestor typed", "Child", "1", ""},
			{"deep ancestor typed", "DeepChild", "1", ""},
			{"private ancestor typed", "PrivateChild", "1", ""},
			{"private null rejection twin", "PrivateChild", "null", "GLADESEMA010"},
			{"private string rejection twin", "PrivateChild", "'value'", "GLADESEMA010"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
					"Base.cls":         `public virtual class Base {public static String pick(Integer value){return 'base';}}`,
					"Child.cls":        `public class Child extends Base {public static String pick(String value){return 'child';}}`,
					"Middle.cls":       `public virtual class Middle extends Base {}`,
					"DeepChild.cls":    `public class DeepChild extends Middle {public static String pick(String value){return 'deep';}}`,
					"PrivateChild.cls": `public class PrivateChild extends Base {private static String pick(String value){return 'private';}}`,
					"Probe.cls":        `public class Probe {public void run(){String value=` + tc.receiver + `.pick(` + tc.call + `);}}`,
				}, api)
				if tc.code == "" && result.HasErrors() {
					t.Fatalf("captured accepted call rejected: %#v", result.Diagnostics)
				}
				if tc.code != "" && (len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != tc.code) {
					t.Fatalf("rejection twin: %#v", result.Diagnostics)
				}
			})
		}
	}
}

// The unary static captures do not establish selection for a multi-argument
// inherited family. Preserve its previous candidate set until it is observed.
func TestResolveMultiArgumentStaticCandidatesRetained(t *testing.T) {
	method := func(left, right string) typesys.MemberSymbol {
		return typesys.MemberSymbol{Kind: apexast.DeclarationMethod, Name: "pick", Type: "String", Modifiers: []string{"public", "static"}, Parameters: []apexast.Parameter{{Name: "left", Type: left}, {Name: "right", Type: right}}}
	}
	index := typesys.Index{Types: []typesys.TypeSymbol{
		{Kind: apexast.DeclarationClass, Name: "Base", Members: []typesys.MemberSymbol{method("String", "Object")}},
		{Kind: apexast.DeclarationClass, Name: "Child", SuperClass: "Base", Members: []typesys.MemberSymbol{method("Object", "String")}},
	}}
	model := buildSemaTypeMemberView(index)
	candidates := applicableResolvedMembers(resolveMemberMethods(model, "Child", "pick"), []string{"null", "null"}, model)
	if len(candidates) != 2 {
		t.Fatalf("uncaptured multi-argument family pruned: %#v", candidates)
	}
}

// Native captures no Iterator/Iterable return specialization. Preserve the platform
// declarations on member lookup; Batchable instantiation is covered separately.
func TestResolveNonBatchableInterfacesKeepPlatformSignatures(t *testing.T) {
	for _, tc := range []struct{ receiver, method string }{
		{"Iterator<Integer>", "next"},
		{"System.Iterator<String>", "next"},
		{"Iterable<Integer>", "iterator"},
		{"System.Iterable<String>", "iterator"},
	} {
		t.Run(tc.receiver, func(t *testing.T) {
			model := buildSemaTypeMemberView(typesys.Index{})
			// Raw member lookup uses the canonical generic base; the empty
			// index has no qualified generic aliases of its own.
			receiver := semaCanonicalPlatformAlias(tc.receiver)
			members, _, ok := semaLookupTypeMembers(model, receiver)
			if !ok || !members.platform {
				t.Fatal("missing platform interface")
			}
			declared := members.methods[normalizeName(tc.method)]
			if len(declared) != 1 {
				t.Fatalf("platform declaration: %#v", declared)
			}
			candidates := resolveMemberMethods(model, receiver, tc.method)
			if len(candidates) != 1 || candidates[0].member.Type != declared[0].Type {
				t.Fatalf("uncaptured return specialization: %#v, declared %s", candidates, declared[0].Type)
			}
		})
	}
}
