package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// assertOnlyLocalNamespaceRejections requires every error to be a local
// "Invalid type" rejection of one of want, and every one of want to reject.
func assertOnlyLocalNamespaceRejections(t *testing.T, route string, ds []diagnostic.Diagnostic, want []string) {
	t.Helper()
	got := map[string]bool{}
	for _, d := range ds {
		if d.Severity != diagnostic.Error {
			continue
		}
		ref, ok := strings.CutPrefix(d.NativeMessage, "Invalid type: ")
		if d.Code != "GLADESEMA006" || !ok || !slices.Contains(want, ref) {
			t.Errorf("%s: unexpected error %s %q | %q", route, d.Code, d.NativeMessage, d.Message)
			continue
		}
		got[ref] = true
	}
	if len(got) != len(want) {
		t.Errorf("%s: rejected=%v, want %v", route, got, want)
	}
}

// Owned synthetic offline boundaries; native controls are cited in the fixture README.
func TestNamespaceAbsenceProofRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/namespace_absence"
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"", "fooNs", "Account"} {
			t.Run(api+"/namespace="+namespace, func(t *testing.T) {
				root := t.TempDir()
				files, err := filepath.Glob(filepath.Join(fixture, "*.cls"))
				if err != nil || len(files) != 11 {
					t.Fatalf("fixture files=%d err=%v", len(files), err)
				}
				var paths []string
				for _, file := range files {
					source, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}
					if namespace == "Account" && filepath.Base(file) == "NamespacePositions.cls" {
						source = []byte(strings.Replace(string(source), "public static void guarded() {", "public static void guarded() { if (false) { Account.Uncatalogued.log(); Object value = new Account.Uncatalogued(); }", 1))
					}
					path := filepath.Join(root, filepath.Base(file))
					writeSemaFile(t, path, string(source))
					writeSemaFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				// v0.3.1 local-declaration rule (A04 C055/N010/N011): only local
				// declarations of unresolved namespace types reject; the other
				// positions in the fixture stay open. With fooNs declared,
				// fooNs.X names the project class X.
				want := []string{"bar.Wrapper", "bar.wrapper", "baz.Type", "fooNs.Unknown", "q.Type"}
				if namespace != "fooNs" {
					want = append(want, "fooNs.X")
				}
				assertOnlyLocalNamespaceRejections(t, "project", Analyze(index).Diagnostics, want)
				source, err := os.ReadFile(filepath.Join(fixture, "anonymous.apex"))
				if err != nil {
					t.Fatal(err)
				}
				assertOnlyLocalNamespaceRejections(t, "anonymous", AnalyzeAnonymous(index, string(source), api).Diagnostics, want)
				for _, source := range []string{"System.NotAType local = null;", "Object value = new System.NotAType();", "Schema.Account.NotAThing local = null;", "Object value = new Schema.Account.NotAThing();"} {
					if r := AnalyzeAnonymous(index, source, api); !r.HasErrors() {
						t.Errorf("absence control accepted: %s", source)
					}
				}
			})
		}
	}
}

func TestNamespaceAbsenceProofPartialAndCollidingCatalogs(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"fooNs", "dep"} {
			t.Run(api+"/"+namespace, func(t *testing.T) {
				root := t.TempDir()
				var paths []string
				for _, name := range []string{namespace, "X"} {
					path := filepath.Join(root, name+".cls")
					writeSemaFile(t, path, "public class "+name+" { public Integer size() { return 1; } }")
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
				if namespace == "fooNs" {
					index.Project.Namespace = namespace
				} else {
					index.Dependencies = []typesys.DependencyInfo{{Namespace: namespace, Status: "missing"}}
				}
				a := NewAnalyzer()
				a.Analyze(index)
				model := buildSemaTypeMemberView(index)
				for _, reference := range []string{namespace + ".Unknown", namespace + ".OtherMissing", namespace + ".X.Missing"} {
					if !semaTypeReferenceKnownAtVersion(a, model, "", reference, api) {
						t.Errorf("namespace/class collision proved absence: %s", reference)
					}
				}
			})
		}
		index := undeclaredNamespaceIndex(t, api, "fooNs", map[string]string{"X": "public class X { public Integer size() { return 1; } }"})
		result := AnalyzeAnonymous(index, "fooNs.X owned = new fooNs.X(); String mismatch = owned.size();", api)
		found := false
		for _, d := range result.Diagnostics {
			found = found || d.NativeMessage == "Illegal assignment from Integer to String"
		}
		if !found {
			t.Fatalf("own namespace did not bind X.size() as Integer: %+v", result.Diagnostics)
		}
	}
}

// System's top-level type inventory is closed; its nested platform-class
// inventories are partial, just like the unqualified platform class spellings.
func TestNamespaceAbsenceProofSystemNestedCatalog(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, shadow := range []bool{false, true} {
			root := t.TempDir()
			var paths []string
			if shadow {
				for _, name := range []string{"String", "Http"} {
					path := filepath.Join(root, name+".cls")
					writeSemaFile(t, path, "public class "+name+" {}")
					paths = append(paths, path)
				}
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
			a := NewAnalyzer()
			a.Analyze(index)
			model := buildSemaTypeMemberView(index)
			for _, reference := range []string{"System.String.UncataloguedType", "System.Http.UncataloguedType"} {
				if !semaTypeReferenceKnownAtVersion(a, model, "", reference, api) {
					t.Errorf("shadow=%v partial nested catalog proved absence: %s", shadow, reference)
				}
				if r := AnalyzeAnonymous(index, reference+" local = null;", api); r.HasErrors() {
					t.Errorf("shadow=%v System nested local: %+v", shadow, r.Diagnostics)
				}
			}
			for _, reference := range []string{"System.NotAType", "System.NotAType.Inner"} {
				if semaTypeReferenceKnownAtVersion(a, model, "", reference, api) {
					t.Errorf("closed System root accepted: %s", reference)
				}
			}
		}
	}
}

func TestNamespaceAbsenceProofInheritedNamespaceCollision(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, namespace := range []string{"fooNs", "dep"} {
			t.Run(api+"/"+namespace, func(t *testing.T) {
				root := t.TempDir()
				var paths []string
				for name, source := range map[string]string{"Base": "public virtual class Base { public class Unknown {} }", namespace: "public class " + namespace + " extends Base {}"} {
					path := filepath.Join(root, name+".cls")
					writeSemaFile(t, path, source)
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
				if namespace == "fooNs" {
					index.Project.Namespace = namespace
				} else {
					index.Dependencies = []typesys.DependencyInfo{{Namespace: namespace, Status: "missing"}}
				}
				if r := AnalyzeAnonymous(index, namespace+".Unknown local = null;", api); r.HasErrors() {
					t.Fatal(r.Diagnostics)
				}
			})
		}
	}
}

func TestNamespaceAbsenceProofClosedDeclarationPositions(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, reference := range []string{"System.NotAType", "Schema.Account.NotAThing"} {
			for _, declaration := range []string{reference + " field;", "public void call(" + reference + " parameter) {}"} {
				root := t.TempDir()
				path := filepath.Join(root, "Absent.cls")
				writeSemaFile(t, path, "public class Absent { "+declaration+" }")
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				if r := Analyze(index); !r.HasErrors() {
					t.Errorf("closed declaration accepted at %s: %s", api, declaration)
				}
			}
		}
	}
}

func TestNamespaceAbsenceProofKnownProjectMembersStayStrict(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := undeclaredNamespaceIndex(t, api, "fooNs", map[string]string{"ProjectValue": "public class ProjectValue { public static ProjectValue Instance; public String present; }"})
		for _, body := range []string{"fooNs.ProjectValue.Instance.Missing.trim();", "ProjectValue.Instance.Missing.trim();"} {
			if r := AnalyzeAnonymous(index, body, api); !r.HasErrors() {
				t.Errorf("complete project member miss accepted: %s", body)
			}
		}
	}
}

// Native captures remain immutable. These rows are offline namespace deferrals,
// not assertions that the capture org compiled the source successfully.
func namespaceAbsenceDeferrals(t *testing.T, dir string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "deferred.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
		name, reason, ok := strings.Cut(line, "\t")
		if !ok || name == "" || reason == "" || result[name] {
			t.Fatalf("invalid deferral row: %q", line)
		}
		result[name] = true
	}
	return result
}

// Directly exercise the text fallback used when body IR is unavailable.
func TestNamespaceAbsenceProofTextExpressionFallback(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Build(project.Project{SourceAPIVersion: api}, schema.Schema{})
		a := NewAnalyzer()
		a.Analyze(index)
		model := buildSemaTypeMemberView(index)
		typ := typesys.TypeSymbol{Name: "Probe", EffectiveAPIVersion: api}
		member := typesys.MemberSymbol{Name: "observe"}
		for _, expr := range []string{"(q.Type) null", "null instanceof bar.wrapper", "true ? (q.Type) null : (bar.wrapper) null"} {
			if ds := a.checkSemaExpressionTypeReferences(typ, member, expr, 0, expr, map[string]bool{}, model); len(ds) != 0 {
				t.Errorf("fallback %s: %+v", expr, ds)
			}
		}
		for _, expr := range []string{"(System.NotAType) null", "null instanceof Schema.Account.NotAThing"} {
			if ds := a.checkSemaExpressionTypeReferences(typ, member, expr, 0, expr, map[string]bool{}, model); len(ds) == 0 {
				t.Errorf("fallback closed control accepted: %s", expr)
			}
		}
	}
}

func TestNamespaceAbsenceProofDuplicateCatalogs(t *testing.T) {
	const fixture = "../../testdata/conformance/namespace_absence_duplicates"
	for _, api := range []string{"62.0", "67.0"} {
		for _, mixed := range []bool{false, true} {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/reverse=%v/mixed=%v", api, reverse, mixed), func(t *testing.T) {
					root := t.TempDir()
					var files []string
					err := filepath.WalkDir(fixture, func(path string, entry os.DirEntry, err error) error {
						if err != nil {
							return err
						}
						if !entry.IsDir() && filepath.Ext(path) == ".cls" {
							files = append(files, path)
						}
						return nil
					})
					if err != nil || len(files) != 10 {
						t.Fatalf("fixture files=%d err=%v", len(files), err)
					}
					var paths []string
					for _, file := range files {
						rel, err := filepath.Rel(fixture, file)
						if err != nil {
							t.Fatal(err)
						}
						source, err := os.ReadFile(file)
						if err != nil {
							t.Fatal(err)
						}
						if mixed && filepath.Base(file) == "CatalogConsumer.cls" {
							source = []byte(strings.Replace(string(source), "public class CatalogConsumer {", "public class CatalogConsumer { public alphaNs.Unknown namespaceField; public static void namespaceGuard() { if (false) { alphaNs.Logger.log(); Object value = new alphaNs.Unknown(); } }", 1))
						}
						path := filepath.Join(root, rel)
						if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
							t.Fatal(err)
						}
						writeSemaFile(t, path, string(source))
						writeSemaFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
						writeSemaFile(t, filepath.Join(root, strings.Split(rel, string(filepath.Separator))[0], "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":".","default":true}]}`)
						paths = append(paths, path)
					}
					if reverse {
						for i, j := 0, len(paths)-1; i < j; i, j = i+1, j-1 {
							paths[i], paths[j] = paths[j], paths[i]
						}
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
					if mixed {
						for i := range index.Types {
							rel, err := filepath.Rel(root, index.Types[i].File)
							if err != nil {
								t.Fatal(err)
							}
							switch strings.Split(rel, string(filepath.Separator))[0] {
							case "a":
								index.Types[i].Namespace = "alphaNs"
							case "b":
								index.Types[i].Namespace = "betaNs"
							}
						}
					}

					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					if r := Analyze(index); r.HasErrors() {
						t.Fatalf("check: %+v", r.Diagnostics)
					}
					if r := AnalyzeAnonymous(index, "CatalogRoot.Unknown local = null; ForeignRoot.Unknown other = null; ForeignChild.Inherited inherited = null; List<CatalogRoot.Unknown> values = new List<CatalogRoot.Unknown>(); if (false) { local = new CatalogRoot.Unknown(); ForeignRoot.Unknown.log(); ForeignRoot.Present.Unknown.log(); }", api); r.HasErrors() {
						t.Fatalf("anonymous: %+v", r.Diagnostics)
					}
					scoped := index
					scoped.Project.Root = filepath.Join(root, "c")
					for _, reference := range []string{"UniqueRoot.Unknown", "SiblingRoot.Unknown"} {
						if r := AnalyzeAnonymous(scoped, reference+" local = null;", api); !r.HasErrors() {
							t.Fatal("local source catalog was opened: " + reference)
						}
					}
				})
			}
		}
	}
}

func TestNamespaceAbsenceProofCatalogEntriesAreNotSourceDeclarations(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := undeclaredNamespaceIndex(t, api, "", map[string]string{
			"CatalogOnly":   "public class CatalogOnly {}",
			"ArtifactShape": "public class ArtifactShape {}",
		})
		for i := range index.Types {
			if index.Types[i].Name == "CatalogOnly" {
				index.Types[i].File = ""
				index.Types[i].SourceBacked = false
			} else {
				index.Types[i].Artifact = true
			}
		}
		a := NewAnalyzer()
		a.prepareAnalysisModel(index)
		model := buildSemaTypeMemberView(index)
		for _, reference := range []string{"CatalogOnly.Unknown", "CatalogOnly.OtherMissing", "ArtifactShape.Unknown", "ArtifactShape.OtherMissing"} {
			if !semaTypeReferenceKnownAtVersion(a, model, "", reference, api) {
				t.Errorf("catalog entry proved source absence: %s", reference)
			}
		}
	}
}

func TestNamespaceAbsenceProofFlatSourceOwnership(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := undeclaredNamespaceIndex(t, api, "", map[string]string{
			"FlatRoot":     "public class FlatRoot {}",
			"FlatConsumer": "public class FlatConsumer { public Object make() { return new FlatRoot.Unknown(); } }",
		})
		if r := Analyze(index); !r.HasErrors() {
			t.Fatal("flat source inventory was opened")
		}
		a := NewAnalyzer()
		a.prepareAnalysisModel(index)
		model := buildSemaTypeMemberView(index)
		for _, reference := range []string{"FlatRoot.Unknown", "FlatRoot.OtherMissing"} {
			if !semaTypeReferenceKnownAtVersion(a, model, "NotIndexedOwner", reference, api) {
				t.Errorf("unresolved named owner proved absence: %s", reference)
			}
		}
	}
}

func TestNamespaceAbsenceProofDependencyStaticReceiver(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: api}, Types: []typesys.TypeSymbol{{Name: "KnownType", Namespace: "dep", Dependency: true}}}
		for _, receiver := range []string{"dep.KnownType.Uncatalogued", "dep.KnownType.OtherMissing"} {
			// The release rejects a static call on an uncatalogued member of a
			// symbol-only dependency namespace; offline execution has no method.
			if r := AnalyzeAnonymous(index, "if (false) { "+receiver+".log(); }", api); !r.HasErrors() {
				t.Errorf("partial dependency receiver %s accepted", receiver)
			}
		}
	}
}

func TestNamespaceAbsenceProofClassTriggerOwnerCollision(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		var paths []string
		for name, source := range map[string]string{
			"a/SharedOwner.cls":     "public class SharedOwner {}",
			"a/ForeignRoot.cls":     "public class ForeignRoot {}",
			"c/SharedOwner.trigger": "trigger SharedOwner on Account (before insert) { if (false) { Object value = new ForeignRoot.Unknown(); ForeignRoot.OtherMissing.log(); } }",
		} {
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			writeSemaFile(t, path, source)
			writeSemaFile(t, filepath.Join(filepath.Dir(path), "sfdx-project.json"), `{"packageDirectories":[{"path":".","default":true}],"sourceApiVersion":"`+api+`"}`)
			paths = append(paths, path)
		}
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
		if r := Analyze(index); r.HasErrors() {
			t.Fatalf("trigger borrowed a foreign class owner: %+v", r.Diagnostics)
		}
	}
}

func TestNamespaceAbsenceProofForeignSystemShadow(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		root := t.TempDir()
		var paths []string
		for name, source := range map[string]string{
			"a/System.cls":   "public class System {}",
			"c/Consumer.cls": "public class Consumer { public static void guarded() { System.Database.Bogus.y(); } }",
		} {
			path := filepath.Join(root, name)
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			writeSemaFile(t, path, source)
			writeSemaFile(t, filepath.Join(filepath.Dir(path), "sfdx-project.json"), `{"packageDirectories":[{"path":".","default":true}],"sourceApiVersion":"`+api+`"}`)
			paths = append(paths, path)
		}
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
		found := false
		for _, d := range Analyze(index).Diagnostics {
			found = found || d.NativeMessage == "Variable does not exist: System.Database.Bogus"
		}
		if !found {
			t.Fatal("foreign System shadow suppressed the captured platform receiver rejection")
		}
	}
}

// Inferred object shapes recognize references permissively; they do not prove
// SObject identity. Declared metadata can establish identity with partial fields.
func TestNamespaceAbsenceProofSObjectIdentityProvenance(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, declared := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/declared=%v", api, declared), func(t *testing.T) {
				objects := []schema.Object{
					{Name: "Provisional__c", Partial: true, Fields: []schema.Field{{Name: "Unknown", Type: "Text"}}},
					{Name: "Other__c", Partial: true, Fields: []schema.Field{{Name: "OtherMissing", Type: "Text"}}},
					{Name: "Http", Partial: true, Fields: []schema.Field{{Name: "UncataloguedType", Type: "Text"}}},
				}
				original := typesys.Index{Project: typesys.ProjectInfo{SourceAPIVersion: api}}
				if declared {
					original.Objects = objects
				}
				a := NewAnalyzer()
				a.prepareAnalysisContext(original, AnalyzeOptions{})
				enriched := original
				enriched.Objects = objects
				a.prepareAnalysisModel(enriched)
				model := buildSemaTypeMemberView(enriched)
				for _, reference := range []string{"Provisional__c.MissingType", "Other__c.MissingType", "Provisional__c.Unknown.UncataloguedType", "Other__c.OtherMissing.Inner", "Schema.Provisional__c.MissingType", "Schema.Other__c.MissingType"} {
					wantType := !declared || !strings.HasPrefix(reference, "Schema.")
					if got := semaTypeReferenceKnownAtVersion(a, model, "", reference, api); got != wantType {
						t.Errorf("type %s accepted=%v, want %v", reference, got, wantType)
					}
					wantReceiver := !declared || (wantType && !strings.Contains(reference, ".Unknown.") && !strings.Contains(reference, ".OtherMissing."))
					if got := semaQualifiedCallReceiverKnown(a, "", reference, newIRSemaScope(nil), model, api); got != wantReceiver {
						t.Errorf("receiver %s accepted=%v, want %v", reference, got, wantReceiver)
					}
				}
				if !semaTypeReferenceKnownAtVersion(a, model, "", "System.Http.UncataloguedType", api) {
					t.Error("object alias closed the explicit System.Http nested catalog")
				}
				for _, receiver := range []string{"System.Http.UncataloguedType", "System.Http.UncataloguedType.Inner"} {
					if !semaQualifiedCallReceiverKnown(a, "", receiver, newIRSemaScope(nil), model, api) {
						t.Errorf("object fields closed the explicit platform receiver: %s", receiver)
					}
				}
				if semaTypeReferenceKnownAtVersion(a, model, "", "Schema.Account.NotAThing", api) {
					t.Error("standard Account identity lost its strict nested-type boundary")
				}
			})
		}
	}
}

// CLI/REST callers request ++/-- lowering permissions after anonymous semantic
// analysis. A deferred qualifier must not alter the captured numeric permission.
func TestNamespaceAbsenceProofAnonymousPrefixPermissions(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		index := typesys.Build(project.Project{SourceAPIVersion: api}, schema.Schema{})
		source := "String value = null; List<Decimal> values = new List<Decimal>{2}; ++values[0];"
		for _, warm := range []bool{false, true} {
			if r := AnalyzeAnonymous(index, source, api); r.HasErrors() {
				t.Fatalf("%s warm=%v: %+v", api, warm, r.Diagnostics)
			}
			approved := ApprovedAnonymousPrefixStatements(index, source, api)
			if len(approved) != 1 || !approved[strings.Index(source, "++")] {
				t.Fatalf("%s warm=%v: %v", api, warm, approved)
			}
			for _, missing := range []string{"bar.Wrapper", "baz.Type"} {
				if r := AnalyzeAnonymous(index, missing+" absent; "+source, api); !r.HasErrors() {
					t.Fatalf("unresolved %s accepted", missing)
				}
			}
		}
	}
}

func TestNamespaceAbsenceProofPartialFormulaCatalog(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := undeclaredNamespaceIndex(t, api, "", map[string]string{
				"FormulaCatalogConsumer": `public abstract class FormulaCatalogConsumer
                    extends FormulaEval.UncataloguedBase implements FormulaEval.UncataloguedInterface {
                    public FormulaEval.Uncatalogued field;
                    public static void inspect(FormulaEval.Uncatalogued parameter) {
                        FormulaEval.Uncatalogued local = new FormulaEval.Uncatalogued();
                        List<FormulaEval.OtherUncatalogued> values = new List<FormulaEval.OtherUncatalogued>();
                        try {} catch (FormulaEval.UncataloguedException caught) {}
                        if (false) { FormulaEval.Uncatalogued.log(); }
                    }
                }`,
			})
			if result := Analyze(index); result.HasErrors() {
				t.Errorf("partial FormulaEval catalog proved absence: %+v", result.Diagnostics)
			}
			for _, reference := range []string{"FormulaEval.Uncatalogued", "FormulaEval.OtherUncatalogued", "FormulaEval.UncataloguedException"} {
				if result := AnalyzeAnonymous(index, reference+" local = null;", api); result.HasErrors() {
					t.Errorf("anonymous partial catalog %s: %+v", reference, result.Diagnostics)
				}
			}
			// Native formula_evaluation C016/C017 reject exactly these spellings.
			for _, reference := range []string{"FormulaEval.FormulaValidationException", "FormulaEval.FormulaEvaluationException"} {
				if result := AnalyzeAnonymous(index, reference+" local = null;", api); !result.HasErrors() {
					t.Errorf("captured FormulaEval exception accepted: %s", reference)
				}
			}
		})
	}
}
