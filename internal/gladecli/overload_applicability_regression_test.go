package gladecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
)

// These owned fixtures retain independent native API62/API67 anonymous and
// named observations. Anonymous sources use the capture's review log adapter;
// the case expressions are unchanged from the native exception transport.
func TestOwnedOverloadApplicability(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	fixtureRoot := filepath.Join("..", "..", "testdata", "local-tests", "overload-applicability")
	var fixture struct {
		APIVersions           []string          `json:"apiVersions"`
		AnonymousSourceSHA256 map[string]string `json:"anonymousSourceSHA256"`
		Runtime               []struct {
			ID     string `json:"id"`
			Method string `json:"method"`
		} `json:"runtime"`
		ThisDispatch     map[string]ownedThisDispatchCapture                `json:"thisDispatch"`
		OverloadDispatch map[string]map[string]ownedOverloadDispatchCapture `json:"overloadDispatch"`
		Rejections       []struct {
			ID   string `json:"id"`
			Code string `json:"code"`
		} `json:"rejections"`
	}
	data, err := os.ReadFile(filepath.Join(fixtureRoot, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.APIVersions) != 2 || fixture.APIVersions[0] != "62.0" || fixture.APIVersions[1] != "67.0" {
		t.Fatalf("unexpected API matrix: %v", fixture.APIVersions)
	}
	if len(fixture.Runtime) != 8 || len(fixture.Rejections) != 7 {
		t.Fatalf("fixture has %d runtime rows and %d rejections; want 8 and 7", len(fixture.Runtime), len(fixture.Rejections))
	}
	if len(fixture.AnonymousSourceSHA256) != 8 {
		t.Fatalf("fixture has %d anonymous source identities, want 8", len(fixture.AnonymousSourceSHA256))
	}
	if len(fixture.ThisDispatch) != 2 {
		t.Fatalf("this_dispatch API matrix: %d captures, want 2", len(fixture.ThisDispatch))
	}
	if len(fixture.OverloadDispatch) != 2 {
		t.Fatalf("overload_dispatch API matrix: %d captures, want 2", len(fixture.OverloadDispatch))
	}
	for _, api := range fixture.APIVersions {
		t.Run(api, func(t *testing.T) {
			t.Run("overload_dispatch", func(t *testing.T) {
				routes := fixture.OverloadDispatch[api]
				if len(routes) != 2 || routes["anonymous"].Route != "ANONYMOUS" || routes["isTest"].Route != "@IsTest" {
					t.Fatal("incomplete overload_dispatch anonymous/@IsTest matrix")
				}
				for _, route := range []string{"anonymous", "isTest"} {
					t.Run(route, func(t *testing.T) {
						ownedOverloadDispatchConformance(t, api, routes[route])
					})
				}
			})
			t.Run("this_dispatch", func(t *testing.T) {
				ownedThisDispatchConformance(t, api, fixture.ThisDispatch[api])
			})
			t.Run("anonymous", func(t *testing.T) {
				anonymousRoot := filepath.Join(fixtureRoot, "anonymous")
				native := ownedOverloadRows(t, filepath.Join(anonymousRoot, "api"+strings.TrimSuffix(api, ".0")+".tsv"))
				if len(native) != len(fixture.Runtime)+len(fixture.Rejections) {
					t.Fatalf("native anonymous rows=%d, want 15", len(native))
				}
				root := t.TempDir()
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				load := loadExecProject(root, false)
				if load.err != nil {
					t.Fatal(load.err)
				}
				if load.index.HasErrors() {
					t.Fatal(load.index.Diagnostics)
				}
				t.Run("runtime", func(t *testing.T) {
					source := ownedOverloadCaptureSource(t, filepath.Join(anonymousRoot, "runtime.apex"), fixture.AnonymousSourceSHA256["runtime"])
					var stdout, stderr bytes.Buffer
					code := Run(context.Background(), []string{"exec", "--project", root, "--json", source}, &stdout, &stderr)
					if code != 0 {
						t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
					}
					var result struct {
						Debug []string `json:"debug"`
					}
					envelope := decodeCLIEnvelopeData(t, stdout.Bytes(), "exec", &result)
					if envelope.Status != "passed" || envelope.ExitCode != 0 {
						t.Fatalf("unexpected anonymous envelope: %+v", envelope)
					}
					seen := make(map[string]string)
					for _, message := range result.Debug {
						parts := strings.SplitN(message, "|", 3)
						if len(parts) != 3 || parts[0] != "P" {
							t.Fatalf("unexpected anonymous observation: %q", message)
						}
						if _, duplicate := seen[parts[1]]; duplicate {
							t.Fatalf("duplicate anonymous row: %s", parts[1])
						}
						seen[parts[1]] = parts[2]
					}
					if len(seen) != len(fixture.Runtime) {
						t.Fatalf("ran %d anonymous rows, want %d", len(seen), len(fixture.Runtime))
					}
					for _, row := range fixture.Runtime {
						want, observed := native[row.ID]
						got, executed := seen[row.ID]
						if !observed || !executed || strings.HasPrefix(want, "COMPILE_ERROR\t") || got != want {
							t.Errorf("anonymous %s: executed=%t got=%q native=%q", row.ID, executed, got, want)
						}
					}
				})
				for _, row := range fixture.Rejections {
					t.Run(row.ID, func(t *testing.T) {
						if !strings.HasPrefix(native[row.ID], "COMPILE_ERROR\t") {
							t.Fatalf("missing native anonymous rejection %s: %q", row.ID, native[row.ID])
						}
						source := ownedOverloadCaptureSource(t, filepath.Join(anonymousRoot, "compile", row.ID+".apex"), fixture.AnonymousSourceSHA256[row.ID])
						var stdout, stderr bytes.Buffer
						code := Run(context.Background(), []string{"exec", "--project", root, "--json", source}, &stdout, &stderr)
						if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), row.Code+":") {
							t.Fatalf("anonymous %s: want %s before execution; exit=%d stderr=%s stdout=%s", row.ID, row.Code, code, stderr.String(), stdout.String())
						}
						prepared, err := prepareAnonymousSourceInContext(source, load.index)
						if err != nil {
							t.Fatal(err)
						}
						defer prepared.close()
						if prepared.apiVersion != api {
							t.Fatalf("prepared API=%q, want %q", prepared.apiVersion, api)
						}
						analysisIndex := mergeAnonymousIndex(load.index, prepared.index)
						analysis := sema.AnalyzeAnonymous(analysisIndex, prepared.body, prepared.apiVersion)
						got := ownedOverloadCompileObservation(t, analysis.Diagnostics, row.Code, true)
						for _, item := range analysis.Diagnostics {
							if item.Severity == diagnostic.Error && stderr.String() != "glade: "+item.Code+": "+item.Message+"\n" {
								t.Fatalf("anonymous CLI did not retain exact local diagnostic %q: %q", item.Message, stderr.String())
							}
						}
						if got != native[row.ID] {
							t.Errorf("anonymous %s: observed=%q native=%q", row.ID, got, native[row.ID])
						}
					})
				}
			})
			t.Run("isTest/runtime", func(t *testing.T) {
				root := ownedOverloadProject(t, filepath.Join(fixtureRoot, "runtime"), "OverloadApplicabilityTest", api)
				var stdout, stderr bytes.Buffer
				code := Run(context.Background(), []string{"test", "--project", root, "--class", "OverloadApplicabilityTest", "--no-cache", "--no-serve", "--no-progress", "--json"}, &stdout, &stderr)
				if code != 0 {
					t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
				}
				run, err := decodeTestRunJSON(stdout.Bytes())
				if err != nil {
					t.Fatalf("decode: %v\n%s", err, stdout.String())
				}
				seen := make(map[string]bool)
				for _, suite := range run.Suites {
					for _, row := range suite.Cases {
						if row.Status != testreport.StatusPass || seen[row.MethodName] {
							t.Fatalf("unexpected runtime result: %+v", row)
						}
						seen[row.MethodName] = true
					}
				}
				if len(seen) != len(fixture.Runtime) {
					t.Fatalf("ran %d methods, want %d", len(seen), len(fixture.Runtime))
				}
				for _, row := range fixture.Runtime {
					if !seen[row.Method] {
						t.Errorf("native row %s method %s did not run", row.ID, row.Method)
					}
				}
			})
			namedRoot := filepath.Join(fixtureRoot, "named", "api"+strings.TrimSuffix(api, ".0"))
			namedNative := ownedOverloadRows(t, filepath.Join(namedRoot, "compile.tsv"))
			namedSources := ownedOverloadNamedSources(t, filepath.Join(namedRoot, "compile-sources.json"))
			if len(namedNative) != len(fixture.Runtime)+len(fixture.Rejections) || len(namedSources) != len(fixture.Rejections) {
				t.Fatalf("incomplete named capture: %d observations and %d compiler sources", len(namedNative), len(namedSources))
			}
			for _, row := range fixture.Rejections {
				t.Run("isTest/"+row.ID, func(t *testing.T) {
					captured, ok := namedSources[row.ID]
					if !ok || !strings.HasPrefix(namedNative[row.ID], "COMPILE_ERROR\t") {
						t.Fatalf("missing named source or rejection for %s", row.ID)
					}
					source := ownedOverloadCaptureSource(t, filepath.Join(namedRoot, captured.Source), captured.SourceSHA256)
					root := ownedOverloadSourceProject(t, source, captured.Name, api)
					var stdout, stderr bytes.Buffer
					code := Run(context.Background(), []string{"check", "--project", root, "--no-cache", "--no-progress", "--json"}, &stdout, &stderr)
					if code != 1 {
						t.Fatalf("exit=%d, want semantic rejection; stderr=%s stdout=%s", code, stderr.String(), stdout.String())
					}
					var envelope struct {
						Diagnostics []diagnostic.Diagnostic `json:"diagnostics"`
					}
					if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
						t.Fatalf("decode: %v\n%s", err, stdout.String())
					}
					// CLI JSON exposes local Message, while the ordinary analyzer
					// also retains NativeMessage for exact oracle comparison.
					local := ownedOverloadCompileObservation(t, envelope.Diagnostics, row.Code, false)
					index, err := loadIndex(root)
					if err != nil {
						t.Fatal(err)
					}
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					analysis := sema.Analyze(index)
					got := ownedOverloadCompileObservation(t, analysis.Diagnostics, row.Code, false)
					for _, item := range analysis.Diagnostics {
						if item.Severity == diagnostic.Error && local != "COMPILE_ERROR\t"+item.Message {
							t.Fatalf("named CLI local diagnostic=%q, analyzer message=%q", local, item.Message)
						}
					}
					if got != namedNative[row.ID] {
						t.Errorf("named %s: observed=%q native=%q", row.ID, got, namedNative[row.ID])
					}
				})
			}
		})
	}
}

// These synthetic cases exercise declared argument and receiver types. In
// particular, this has the type of the class containing the executing method,
// and an interface receiver exposes only that interface's overloads.
func TestOverloadApplicabilityStaticThisAndInterface(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	const source = `@IsTest private class StaticDispatchTest {
    interface NodeVisitor {
        Object visit(NumberNode node);
        Object visit(SumNode node);
    }
    abstract class Node {
        public abstract Object accept(NodeVisitor visitor);
    }
    class NumberNode extends Node {
        Integer value;
        NumberNode(Integer value) { this.value = value; }
        public override Object accept(NodeVisitor visitor) { return visitor.visit(this); }
    }
    class SumNode extends Node {
        Node left;
        Node right;
        SumNode(Node left, Node right) { this.left = left; this.right = right; }
        public override Object accept(NodeVisitor visitor) { return visitor.visit(this); }
    }
    class Evaluator implements NodeVisitor {
        public Object visit(NumberNode node) { return this.numberValue(node); }
        private Integer numberValue(NumberNode node) { return node.value; }
        public Object visit(SumNode node) {
            return (Integer)node.left.accept(this) + (Integer)node.right.accept(this);
        }
    }
    virtual class BaseValue {
        public String chooseSelf(Selector selector) { return selector.choose(this); }
    }
    class ChildValue extends BaseValue {
        public void register(Registry registry) { registry.setValue(this); }
    }
    class Registry {
        ChildValue saved;
        public void setValue(ChildValue value) { saved = value; }
    }
    interface BaseSelector { String choose(BaseValue value); }
    class Selector implements BaseSelector {
        public String choose(BaseValue value) { return 'base'; }
        public String choose(ChildValue value) { return 'child'; }
    }
    interface ParentSelector { String selectValue(BaseValue value); }
    interface ChildSelector extends ParentSelector { String selectValue(ChildValue value); }
    virtual class ParentSelectorImpl implements ParentSelector {
        public String selectValue(BaseValue value) { return 'parent'; }
    }
    class ChildSelectorImpl extends ParentSelectorImpl implements ChildSelector {
        public String selectValue(ChildValue value) { return 'child'; }
        public String selectValue(String value) { return 'string'; }
    }
    interface ValueMarker {}
    class MarkedValue implements ValueMarker {
        public override String toString() { return 'marked'; }
        public Integer hashCode() { return 37; }
        public Boolean equals(Object other) { return true; }
        public Boolean equals(MarkedValue other) { return false; }
    }
    @IsTest static void visitorDispatch() {
        Node expression = new SumNode(new NumberNode(8), new NumberNode(13));
        NodeVisitor visitor = new Evaluator();
        System.assertEquals(21, expression.accept(visitor));
    }
    @IsTest static void subclassThisArgument() {
        Registry registry = new Registry();
        ChildValue child = new ChildValue();
        child.register(registry);
        System.assertEquals(child, registry.saved);
    }
    @IsTest static void inheritedThisArgument() {
        System.assertEquals('base', new ChildValue().chooseSelf(new Selector()));
    }
    @IsTest static void interfaceOverloadSet() {
        ChildValue child = new ChildValue();
        BaseSelector selector = new Selector();
        System.assertEquals('base', selector.choose(child));
        System.assertEquals('child', new Selector().choose(child));
    }
    @IsTest static void inheritedInterfaceOverloadSet() {
        ChildSelector selector = new ChildSelectorImpl();
        BaseValue baseValue = new ChildValue();
        ChildValue child = new ChildValue();
        System.assertEquals('parent', selector.selectValue(baseValue));
        System.assertEquals('child', selector.selectValue(child));
        ParentSelector parent = selector;
        System.assertEquals('parent', parent.selectValue(child));
    }
    @IsTest static void interfaceObjectMethods() {
        MarkedValue concrete = new MarkedValue();
        ValueMarker marker = concrete;
        MarkedValue other = new MarkedValue();
        System.assertEquals('marked', marker.toString());
        System.assertEquals(37, marker.hashCode());
        System.assertEquals(true, marker.equals(other));
        System.assertEquals(false, concrete.equals(other));
    }
}`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := ownedOverloadSourceProject(t, source, "StaticDispatchTest", api)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"test", "--project", root, "--class", "StaticDispatchTest", "--no-cache", "--no-serve", "--no-progress", "--json"}, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit=%d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
			}
			run, err := decodeTestRunJSON(stdout.Bytes())
			if err != nil {
				t.Fatalf("decode: %v\n%s", err, stdout.String())
			}
			seen := make(map[string]bool)
			for _, suite := range run.Suites {
				for _, row := range suite.Cases {
					if row.Status != testreport.StatusPass || seen[row.MethodName] {
						t.Fatalf("unexpected runtime result: %+v", row)
					}
					seen[row.MethodName] = true
				}
			}
			for _, name := range []string{"visitorDispatch", "subclassThisArgument", "inheritedThisArgument", "interfaceOverloadSet", "inheritedInterfaceOverloadSet", "interfaceObjectMethods"} {
				if !seen[name] {
					t.Errorf("method %s did not run", name)
				}
			}
			if len(seen) != 6 {
				t.Fatalf("ran %d methods, want 6", len(seen))
			}
		})
	}
}

// The rejection twins retain the capture's lexical base-class this on both
// routes, including the exact API-specific named source and native signature.
func TestOverloadApplicabilityStaticThisRejection(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	var fixture struct {
		ThisDispatch map[string]ownedThisDispatchCapture `json:"thisDispatch"`
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "local-tests", "overload-applicability", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			ownedThisDispatchRejection(t, api, fixture.ThisDispatch[api])
		})
	}
}

func ownedOverloadRows(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	rows := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		id, observed, ok := strings.Cut(line, "\t")
		if !ok || id == "" || observed == "" {
			t.Fatalf("malformed native row in %s: %q", path, line)
		}
		if _, duplicate := rows[id]; duplicate {
			t.Fatalf("duplicate native row in %s: %s", path, id)
		}
		rows[id] = observed
	}
	return rows
}

func ownedOverloadCaptureSource(t *testing.T, path, expectedSHA256 string) string {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(source)); got != expectedSHA256 {
		t.Fatalf("source differs from native capture export: %s sha256=%s want=%s", path, got, expectedSHA256)
	}
	return string(source)
}

func ownedOverloadProject(t *testing.T, fixtureRoot, className, api string) string {
	t.Helper()
	source, err := os.ReadFile(filepath.Join(fixtureRoot, "force-app", "main", "default", "classes", className+".cls"))
	if err != nil {
		t.Fatal(err)
	}
	return ownedOverloadSourceProject(t, string(source), className, api)
}

func ownedOverloadSourceProject(t *testing.T, source, className, api string) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
	classFile := filepath.Join(root, "force-app", "main", "default", "classes", className+".cls")
	writeTestFile(t, classFile, source)
	writeTestFile(t, classFile+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
	return root
}

type ownedOverloadNamedSource struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Source       string `json:"source"`
	SourceSHA256 string `json:"sourceSHA256"`
}

func ownedOverloadNamedSources(t *testing.T, path string) map[string]ownedOverloadNamedSource {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var sources []ownedOverloadNamedSource
	if err := json.Unmarshal(data, &sources); err != nil {
		t.Fatal(err)
	}
	byID := make(map[string]ownedOverloadNamedSource)
	for _, source := range sources {
		if source.ID == "" || source.Name == "" || source.Source == "" || source.SourceSHA256 == "" {
			t.Fatalf("incomplete captured named source: %+v", source)
		}
		if _, duplicate := byID[source.ID]; duplicate {
			t.Fatalf("duplicate named source: %s", source.ID)
		}
		byID[source.ID] = source
	}
	return byID
}

func ownedOverloadCompileObservation(t *testing.T, diagnostics []diagnostic.Diagnostic, code string, anonymous bool) string {
	t.Helper()
	var errors []diagnostic.Diagnostic
	for _, item := range diagnostics {
		if item.Severity == diagnostic.Error {
			errors = append(errors, item)
		}
	}
	if len(errors) != 1 || errors[0].Code != code {
		t.Fatalf("want exactly one %s error: %+v", code, diagnostics)
	}
	item := errors[0]
	message := item.NativeMessage
	if message == "" {
		message = item.Message
	}
	if anonymous {
		line := 0
		if item.Range != nil {
			line = item.Range.Start.Line
		}
		if item.NativeLine != nil {
			line = *item.NativeLine
		}
		message = fmt.Sprintf("line %d: %s", line, message)
	}
	return "COMPILE_ERROR\t" + message
}
