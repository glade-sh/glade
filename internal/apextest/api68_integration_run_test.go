package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// api68IntegrationProductClass is an @IntegrationTest class whose @TearDown
// raises, so the report shows that teardown ran after each passing method. A
// failing method keeps its own failure. It is a product-path probe, not a
// native row.
const api68IntegrationProductClass = `@IntegrationTest
private class CapIntegrationRun {
    static String marker;
    @BeforeClass static void setupOnce() {
        insert new Account(Name = 'S68-SHARED-FIXTURE');
    }
    @IntegrationTest static void first() {
        marker = 'first';
        System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'S68-SHARED-FIXTURE' WITH SYSTEM_MODE]);
    }
    @IntegrationTest static void second() {
        marker = 'second';
        System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name = 'S68-SHARED-FIXTURE' WITH SYSTEM_MODE]);
    }
    @IntegrationTest static void failing() {
        marker = 'failing';
        System.assertEquals(2, [SELECT COUNT() FROM Account WHERE Name = 'S68-SHARED-FIXTURE' WITH SYSTEM_MODE]);
    }
    @TearDown static void afterEach() {
        throw new DmlException('teardown after ' + marker);
    }
}`

func api68IntegrationProductRun(t *testing.T, features string) testreport.Run {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"68.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer","features":[`+features+`]}`)
	path := filepath.Join(root, "force-app", "main", "default", "classes", "CapIntegrationRun.cls")
	writeFile(t, path, api68IntegrationProductClass)
	writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>68.0</apiVersion><status>Active</status></ApexClass>`)
	return Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
}

func api68RunCases(run testreport.Run) []testreport.Case {
	var cases []testreport.Case
	for _, suite := range run.Suites {
		cases = append(cases, suite.Cases...)
	}
	return cases
}

// With ApexIntegrationTests on, glade test discovers each @IntegrationTest
// method, runs @BeforeClass as data setup visible to every method and runs
// @TearDown after each passing method. The report does not show whether
// teardown also ran after the failing method; its own failure is kept.
func TestAPI68IntegrationTestsProductPath(t *testing.T) {
	cases := api68RunCases(api68IntegrationProductRun(t, `"ApexIntegrationTests"`))
	if len(cases) != 3 {
		t.Fatalf("cases = %+v, want first, second and failing", cases)
	}
	for _, c := range cases {
		method := strings.ToLower(c.MethodName)
		if c.Status != testreport.StatusFail || c.Problem == nil {
			t.Fatalf("%s: %+v, want a failure", c.Name, c)
		}
		want := "teardown after " + method
		if method == "failing" {
			want = "Expected: 2, Actual: 1"
		}
		// The passing methods saw the BeforeClass row (COUNT 1), so their
		// only failure is the teardown exception.
		if !strings.Contains(c.Problem.Message, want) {
			t.Errorf("%s: message %q, want %q", c.Name, c.Problem.Message, want)
		}
	}
}

// Without the feature, @IntegrationTest methods are not tests: nothing is
// discovered and @BeforeClass is not setup.
func TestAPI68IntegrationTestsProductPathFeatureOff(t *testing.T) {
	cases := api68RunCases(api68IntegrationProductRun(t, ""))
	for _, c := range cases {
		if c.Status == testreport.StatusPass || c.Problem != nil && strings.Contains(c.Problem.Message, "teardown after") {
			t.Errorf("feature off ran %+v", c)
		}
	}
}

// @BeforeClass is setup only in an @IntegrationTest class. In a plain @IsTest
// class, with the feature on, it is not run before the test method.
func TestAPI68BeforeClassNotSetupInPlainTestClass(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"68.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "config", "project-scratch-def.json"), `{"edition":"Developer","features":["ApexIntegrationTests"]}`)
	path := filepath.Join(root, "force-app", "main", "default", "classes", "PlainBeforeClass.cls")
	writeFile(t, path, `@IsTest
private class PlainBeforeClass {
    @BeforeClass static void once() { insert new Account(Name = 'S68-NOT-SETUP'); }
    @IsTest static void run() {
        System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name = 'S68-NOT-SETUP']);
    }
}`)
	writeFile(t, path+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>68.0</apiVersion><status>Active</status></ApexClass>`)
	cases := api68RunCases(Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1}))
	if len(cases) != 1 || cases[0].Status != testreport.StatusPass {
		t.Fatalf("cases = %+v, want one pass", cases)
	}
}
