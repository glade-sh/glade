package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
)

// Require each inner-type block to execute in the combined project, rather than
// relying on the same shape passing in an isolated native conformance fixture.
// Native: conformance/inner_type_shapes/org.tsv:18-21,34-41,50-53,64-67,72-73;
// sema/testdata/inner_type_scope/org.tsv:2-3 (unrelated platform receiver).
func TestCorpusShapeInnerBlocks(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run("API"+api, func(t *testing.T) {
			index := loadTestIndex(t, copyCorpusShapeProject(t, api))
			if len(index.Diagnostics) != 0 {
				t.Fatal(index.Diagnostics)
			}
			if result := sema.Analyze(index); len(result.Diagnostics) != 0 {
				t.Fatal(result.Diagnostics)
			}
			for _, method := range []string{"innerSubclassTargets", "unrelatedEventBus", "duplicateInnerNames", "systemNamedEnumConstant", "testInnerSubclass"} {
				t.Run(method, func(t *testing.T) {
					run := Run(index, Options{SelectedClasses: []string{"ShapeProjectTest"}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
					if s := run.Summary(); s.Total != 1 || s.Passed != 1 || s.CompileErrors != 0 {
						t.Fatalf("inner-type block did not pass: %+v; %s", s, firstRunProblem(run))
					}
				})
			}
			t.Run("anonymous", func(t *testing.T) {
				// The private @IsTest Fake declaration has its own runner route.
				const source = `System.assert(new ShapeInnerTargets.Leaf().matches());
System.assert(ShapeInnerTargets.castTarget(new ShapeInnerTargets.Leaf()));
System.assert(new ShapeInnerPeer.Leaf().matches());
System.assertEquals(0, ShapePublisher.publishEmpty());
System.assertEquals(0, EventBus.publish(new List<SObject>()).size());
System.assertEquals('B', ShapeShadow.typeName());`
				prepared := innerShapePrepareAnonymous(t, api, index, source)
				if len(prepared.diagnostics) != 0 {
					t.Fatal(prepared.diagnostics)
				}
				program, err := vm.CompileAnonymousWithOptions(prepared.body, vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				if _, err := runner.execute(program); err != nil {
					t.Fatal(err)
				}
			})
		})
	}
}

// The native field/enum collision is a rejection, so keep it outside the valid
// deployable project. Native: inner_type_shapes/org.tsv:58-63, API 62/67 named
// and @IsTest: "Illegal forward reference: level". Renaming the field removes
// the collision and must make this block's rejection check fail.
func checkCorpusShapeFieldEnumCollision(t *testing.T, api string) {
	t.Helper()
	for _, collision := range []bool{true, false} {
		root := copyCorpusShapeProject(t, api)
		source := `public class ShapeFieldCollision { public enum Level {LOW} public static Level level = Level.LOW; public static String f(){ return level.name(); } }`
		if !collision {
			source = strings.ReplaceAll(source, "Level level", "Level current")
			source = strings.ReplaceAll(source, "return level.name()", "return current.name()")
		}
		classes := filepath.Join(root, "force-app/main/default/classes")
		writeFile(t, filepath.Join(classes, "ShapeFieldCollision.cls"), source)
		writeFile(t, filepath.Join(classes, "ShapeFieldCollision.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
		index := loadTestIndex(t, root)
		if len(index.Diagnostics) != 0 {
			t.Fatal(index.Diagnostics)
		}
		result := sema.Analyze(index)
		if !collision {
			if len(result.Diagnostics) != 0 {
				t.Fatal(result.Diagnostics)
			}
			t.Log("DETECTED field rename: native rejection no longer present")
			continue
		}
		if len(result.Diagnostics) != 1 || result.Diagnostics[0].NativeMessage != "Illegal forward reference: level" {
			t.Fatalf("expected native field collision rejection: %v", result.Diagnostics)
		}
		// Replay the captured @IsTest declaration as well as the named class.
		testSource := strings.Replace(source, "public class ShapeFieldCollision", "@IsTest private class ShapeFieldCollision", 1)
		testSource = strings.TrimSuffix(testSource, "}") + ` @IsTest static void observe(){ System.debug(f()); } }`
		writeFile(t, filepath.Join(classes, "ShapeFieldCollision.cls"), testSource)
		index = loadTestIndex(t, root)
		run := Run(index, Options{SelectedClasses: []string{"ShapeFieldCollision"}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
		if s := run.Summary(); s.Total != 1 || s.CompileErrors != 1 || !strings.Contains(firstRunProblem(run), "level") {
			t.Fatalf("runner must reject field collision: %+v; %s", s, firstRunProblem(run))
		}
	}
}

func TestCorpusShapeInnerBlockMutations(t *testing.T) {
	const classes = "force-app/main/default/classes/"
	control := loadTestIndex(t, copyCorpusShapeProject(t, "67.0"))
	for _, m := range []struct{ name, file, before, after, method string }{
		{"simpleInstanceof", "ShapeInnerTargets.cls", "this instanceof Leaf", "!(this instanceof Leaf)", "innerSubclassTargets"},
		{"simpleCast", "ShapeInnerTargets.cls", "((Leaf) value) != null", "((Leaf) value) == null", "innerSubclassTargets"},
		{"foreignEventBus", "ShapePublisher.cls", "EventBus.publish", "ShapeForeign.EventBus.publish", "unrelatedEventBus"},
		{"duplicateInnerNames", "ShapeInnerPeer.cls", "this instanceof Leaf", "!(this instanceof Leaf)", "duplicateInnerNames"},
		{"systemEnumConstant", "ShapeShadow.cls", "Type.B.name()", "Type.A.name()", "systemNamedEnumConstant"},
		{"testInnerSubclass", "ShapeProjectTest.cls", "new Fake()", "new ShapeInnerTargets.Leaf()", "testInnerSubclass"},
	} {
		t.Run(m.name, func(t *testing.T) {
			options := Options{SelectedClasses: []string{"ShapeProjectTest"}, SelectedMethod: m.method, NoDiskCache: true, Parallelism: 1}
			if run := Run(control, options); run.Summary().Total != 1 || run.Summary().Passed != 1 {
				t.Fatalf("unmutated block failed: %+v; %s", run.Summary(), firstRunProblem(run))
			}
			root := copyCorpusShapeProject(t, "67.0")
			path := filepath.Join(root, classes, m.file)
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(source), m.before) {
				t.Fatal("mutation target missing")
			}
			writeFile(t, path, strings.ReplaceAll(string(source), m.before, m.after))
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if m.name == "foreignEventBus" {
				for _, d := range analysis.Diagnostics {
					if strings.Contains(d.Message, "publish") && strings.Contains(d.Message, "EventBus") {
						t.Logf("DETECTED by check: %s", d.Message)
						return
					}
				}
				t.Fatalf("missing foreign inner receiver rejection: %v", analysis.Diagnostics)
			}
			if len(analysis.Diagnostics) != 0 {
				t.Fatal(analysis.Diagnostics)
			}
			run := Run(index, options)
			if s := run.Summary(); s.Total != 1 || s.Failed != 1 || s.Passed != 0 || s.CompileErrors != 0 || s.Errors != 0 || s.Unsupported != 0 {
				t.Fatalf("MISSED mutation: %+v; %s", s, firstRunProblem(run))
			}
			item := run.Suites[0].Cases[0]
			if item.Status != testreport.StatusFail || item.Reason != testreport.ReasonAssertion {
				t.Fatalf("expected assertion failure: %+v", item)
			}
			t.Logf("DETECTED by runner: %s", firstRunProblem(run))
		})
	}
}
