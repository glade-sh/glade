package apextest

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

const platformNamespaceRoutesMode = "GLADE_PLATFORM_NAMESPACE_ROUTES"

// The ConnectApi and Slack stubs stay out of the platform view until the
// process names them, and a running VM late-binds them. Each route must give
// the outcomes the ungated runtime gives: dynamic Type.forName (both forms),
// newInstance, typed JSON, instanceof, describe, a missing qualified name, and
// simple-name lookups of gated types written without the namespace word. Each
// mode runs in a fresh process, so the first project is the first of its
// process; the dynamic and bare projects run again after the requests.
func TestPlatformNamespaceLookupRoutesMatchUngated(t *testing.T) {
	if mode := os.Getenv(platformNamespaceRoutesMode); mode != "" {
		if strings.HasPrefix(mode, "ungated") {
			typesys.SetPlatformGateDisabled(true)
		} else if _, excluded := typesys.StandardPlatformSymbolViewWithExcluded(); excluded.Len() == 0 {
			t.Fatal("gated process starts with an ungated view")
		}
		t.Log("DIGEST " + platformNamespaceRoutesDigest(t, strings.HasSuffix(mode, "bare-first")))
		return
	}
	digest := func(mode string) string {
		cmd := exec.Command(os.Args[0], "-test.run=^TestPlatformNamespaceLookupRoutesMatchUngated$", "-test.v")
		cmd.Env = append(os.Environ(), platformNamespaceRoutesMode+"="+mode)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s subprocess: %v\n%s", mode, err, output)
		}
		for _, line := range strings.Split(string(output), "\n") {
			if _, value, ok := strings.Cut(line, "DIGEST "); ok {
				return value
			}
		}
		t.Fatalf("%s subprocess printed no digest:\n%s", mode, output)
		return ""
	}
	for _, order := range []string{"dynamic-first", "bare-first"} {
		gated, ungated := digest("gated-"+order), digest("ungated-"+order)
		if gated != ungated {
			t.Fatalf("%s: gated outcomes differ from ungated\ngated:   %s\nungated: %s", order, gated, ungated)
		}
		if !strings.Contains(gated, "describeStillWorks:pass") {
			t.Fatalf("%s: describe case did not pass: %s", order, gated)
		}
	}
}

func platformNamespaceRoutesDigest(t *testing.T, bareFirst bool) string {
	enumName, enumValue, className := gatedRouteNames(t)
	sfdx := `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`
	dynamic := newSalesforceSurfaceProject(t, sfdx)
	dynamic.writeClass("DynamicNamespaceTest", `
@IsTest private class DynamicNamespaceTest {
  static String connect() { return 'Connect' + 'Api'; }
  static String sl() { return 'Sla' + 'ck'; }
  static String text(Object value) { return value == null ? 'null' : String.valueOf(value); }
  @IsTest static void qualifiedForName() {
    Type qualified = Type.forName(connect() + '.FeedBody');
    Type separated = Type.forName(connect(), 'FeedBody');
    System.assert(false, 'R1 ' + text(qualified) + ' ' + text(separated) + ' ' + (qualified == separated));
  }
  @IsTest static void newInstance() {
    Type qualified = Type.forName(connect() + '.FeedItemInput');
    Object value = qualified.newInstance();
    System.assert(false, 'R2 ' + text(qualified) + ' ' + text(value) + ' ' + qualified.getName());
  }
  @IsTest static void typedJSON() {
    Type qualified = Type.forName(connect() + '.FeedItemInput');
    Object value = JSON.deserialize('{}', qualified);
    System.assert(false, 'R3 ' + text(value) + ' ' + JSON.serialize(value));
  }
  @IsTest static void slackForName() {
    System.assert(false, 'R4 ' + text(Type.forName(sl() + '.App')) + ' ' + text(Type.forName(sl(), 'App')) + ' ' + text(Type.forName(sl(), 'DefinitelyMissing')));
  }
  @IsTest static void slackInstance() {
    Object value = Type.forName(sl() + '.App').newInstance();
    System.assert(false, 'R5 ' + text(value));
  }
  @IsTest static void describeStillWorks() {
    System.assertEquals('Account', Account.SObjectType.getDescribe().getName());
  }
  @IsTest static void simpleNames() {
    System.assert(false, 'R6 ' + text(Type.forName('`+className+`')) + ' ' + text(Type.forName('`+enumName+`')));
  }
}`)
	bare := newSalesforceSurfaceProject(t, sfdx)
	var bareEnums strings.Builder
	for i, ref := range gatedRouteEnumValues(t, 8) {
		fmt.Fprintf(&bareEnums, "  @IsTest static void enum%d() { Object value = %s; System.assert(false, 'B1 ' + value); }\n", i, ref)
		fmt.Fprintf(&bareEnums, "  @IsTest static void values%d() { System.assert(false, 'B3 ' + %s.values()); }\n", i, strings.Split(ref, ".")[0])
	}
	bare.writeClass("BareGatedEnumTest", `
@IsTest private class BareGatedEnumTest {
  @IsTest static void enumValue() {
    Object value = `+enumName+`.`+enumValue+`;
    System.assert(false, 'B1 ' + value);
  }
`+bareEnums.String()+`}`)
	bareConstruct := newSalesforceSurfaceProject(t, sfdx)
	bareConstruct.writeClass("BareGatedConstructTest", `
@IsTest private class BareGatedConstructTest {
  @IsTest static void construct() {
    Object value = new `+className+`();
    System.assert(false, 'B2 ' + value);
  }
}`)
	static := newSalesforceSurfaceProject(t, sfdx)
	static.writeClass("StaticNamespaceTest", `
@IsTest private class StaticNamespaceTest {
  @IsTest static void check() {
    Object value = new ConnectApi.FeedItemInput();
    System.assert(value instanceof ConnectApi.FeedItemInput);
    System.assert(false, 'S1 ' + (value instanceof ConnectApi.FeedBody) + ' ' + Type.forName('Slack.DefinitelyMissing'));
  }
}`)
	type step struct {
		name    string
		project salesforceSurfaceProject
	}
	steps := []step{{"dynamic", dynamic}, {"bare", bare}, {"bare-construct", bareConstruct}, {"dynamic-again", dynamic}, {"static", static}, {"bare-again", bare}}
	if bareFirst {
		steps = []step{{"bare", bare}, {"bare-construct", bareConstruct}, {"dynamic", dynamic}, {"bare-again", bare}, {"static", static}, {"dynamic-again", dynamic}}
	}
	var parts []string
	for _, step := range steps {
		digest := platformRouteRunDigest(step.project.run())
		// Each mode runs in its own temp root; compare paths below it.
		digest = strings.ReplaceAll(digest, step.project.root, "<root>")
		parts = append(parts, step.name+"="+digest)
	}
	return strings.Join(parts, " | ")
}

// gatedRouteNames picks, in stub-table order, the first top-level gated enum
// with a value and the first Input class, by simple name. The names never
// contain a namespace word.
func gatedRouteNames(t *testing.T) (enumName, enumValue, className string) {
	t.Helper()
	var names []string
	for _, word := range []string{"ConnectApi", "Slack"} {
		names = append(names, typesys.GatedPlatformStubRuntimeNamesInNamespace(word)...)
	}
	sort.Strings(names)
	for _, runtimeName := range names {
		symbol, ok := typesys.GatedPlatformStubSymbol(runtimeName)
		if !ok {
			continue
		}
		switch symbol.Kind {
		case apexast.DeclarationEnum:
			for _, member := range symbol.Members {
				if enumName == "" && member.Kind != apexast.DeclarationMethod {
					enumName, enumValue = symbol.Name, member.Name
				}
			}
		case apexast.DeclarationClass:
			if className == "" && strings.HasSuffix(symbol.Name, "Input") {
				className = symbol.Name
			}
		}
	}
	if enumName == "" || className == "" {
		t.Fatal("no gated stub enum or Input class")
	}
	return enumName, enumValue, className
}

// gatedRouteEnumValues returns EnumName.VALUE for up to n gated stub enums
// spread over the stub table, by simple name.
func gatedRouteEnumValues(t *testing.T, n int) []string {
	t.Helper()
	var names []string
	for _, word := range []string{"ConnectApi", "Slack"} {
		names = append(names, typesys.GatedPlatformStubRuntimeNamesInNamespace(word)...)
	}
	sort.Strings(names)
	var enums []string
	for _, runtimeName := range names {
		symbol, ok := typesys.GatedPlatformStubSymbol(runtimeName)
		if !ok || symbol.Kind != apexast.DeclarationEnum {
			continue
		}
		for _, member := range symbol.Members {
			if member.Kind != apexast.DeclarationMethod {
				enums = append(enums, symbol.Name+"."+member.Name)
				break
			}
		}
	}
	var out []string
	for i := 0; i < len(enums) && len(out) < n; i += len(enums)/n + 1 {
		out = append(out, enums[i])
	}
	return out
}

func platformRouteRunDigest(run testreport.Run) string {
	var cases []string
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			text := c.ClassName + "." + c.MethodName + ":" + string(c.Status)
			if c.Problem != nil {
				text += ":" + c.Problem.Type + ":" + c.Problem.Message
			}
			cases = append(cases, text)
		}
	}
	sort.Strings(cases)
	return strings.Join(cases, ";")
}

// A namespace request does not change the compiled-runtime key: a runtime
// cached under the gated view late-binds the stubs it lacks, as the ungated
// runtime held them.
func TestPlatformNamespaceRequestKeepsRuntimeKey(t *testing.T) {
	if os.Getenv("GLADE_PLATFORM_NAMESPACE_KEY") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestPlatformNamespaceRequestKeepsRuntimeKey$")
		cmd.Env = append(os.Environ(), "GLADE_PLATFORM_NAMESPACE_KEY=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("cold subprocess: %v\n%s", err, output)
		}
		return
	}
	project := newSalesforceSurfaceProject(t, `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	project.writeClass("KeyTest", `@IsTest private class KeyTest { @IsTest static void check() {} }`)
	index := loadTestIndex(t, project.root)
	initial := RuntimeContentKey(index, nil)
	typesys.NoteApexSource("Slack.App")
	typesys.NoteApexSource("ConnectApi.FeedItem")
	if got := RuntimeContentKey(index, nil); got != initial {
		t.Fatal("a namespace request changed the runtime key")
	}
}

// A late-bound gated class is the class the passive templates of the stub-only
// view build for it.
func TestGatedPlatformStubClassMatchesPassiveTemplate(t *testing.T) {
	const child = "GLADE_GATED_STUB_TEMPLATE_TEST"
	if os.Getenv(child) == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestGatedPlatformStubClassMatchesPassiveTemplate$", "-test.v")
		cmd.Env = append(childEnvWithoutShardSelector(), child+"=1")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("stub-only subprocess: %v\n%s", err, output)
		}
		if !strings.Contains(string(output), "=== RUN   TestGatedPlatformStubClassMatchesPassiveTemplate\n") {
			t.Fatalf("stub-only subprocess did not run the test:\n%s", output)
		}
		return
	}
	templates := buildPassiveStandardRuntimeClassTemplates(typesys.StandardPlatformSymbols())
	checked := 0
	for _, template := range templates {
		if !typesys.IsGatedPlatformTypeName(template.class.Name) {
			continue
		}
		got, ok := gatedPlatformStubClass(template.class.Name)
		if !ok {
			t.Fatalf("no late-bound class for %s", template.class.Name)
		}
		want := template.class.FreshClass()
		if !reflect.DeepEqual(got.FreshClass(), want) {
			t.Fatalf("late-bound %s differs from its passive template:\ngot  %+v\nwant %+v", template.class.Name, got.FreshClass(), want)
		}
		checked++
	}
	if checked < 5000 {
		t.Fatalf("checked only %d gated classes", checked)
	}
}
