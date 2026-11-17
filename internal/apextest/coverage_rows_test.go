package apextest

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

// Methods a failed @TestSetup kept from running get no coverage rows; a test
// that ran keeps its rows, and every trigger row once expanded.
func TestCoverageRowsOnlyForMethodsThatRan(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	for name, body := range map[string]string{
		"Helper":           "public class Helper {\n    public static Integer one() {\n        return 1;\n    }\n}",
		"BlockedSetupTest": "@IsTest private class BlockedSetupTest {\n    @TestSetup static void seed() { Helper.one(); System.assert(false, 'setup'); }\n    @IsTest static void never() { Helper.one(); }\n}",
		"RanTest":          "@IsTest private class RanTest {\n    @IsTest static void ran() { System.assertEquals(1, Helper.one()); }\n}",
	} {
		writeFile(t, filepath.Join(root, "force-app/classes", name+".cls"), body)
	}
	writeFile(t, filepath.Join(root, "force-app/triggers/AccountMark.trigger"), "trigger AccountMark on Account (before insert) {\n    Integer n = 1;\n}")
	run := testreport.NativePerTestRows(Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1, CodeCoverage: true}))
	seen := 0
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			seen++
			switch c.ClassName + "." + c.MethodName {
			case "BlockedSetupTest.never":
				if c.Status != testreport.StatusFail || c.Coverage != nil {
					t.Fatalf("blocked case status=%s rows=%v", c.Status, c.Coverage)
				}
			case "RanTest.ran":
				if got := fmt.Sprint(rowNames(c.Coverage)); got != "[Helper AccountMark]" {
					t.Fatalf("ran rows %v", got)
				}
			}
		}
	}
	if seen != 2 {
		t.Fatalf("cases %d", seen)
	}
}

func rowNames(rows []testreport.TestCoverage) []string {
	var out []string
	for _, r := range rows {
		out = append(out, r.Name)
	}
	return out
}

// Same-named classes from two source units keep their own per-test rows.
func TestCoverageRowsSameNamedClassesStayApart(t *testing.T) {
	root := t.TempDir()
	for ordinal := 1; ordinal <= 2; ordinal++ {
		base := filepath.Join(root, fmt.Sprint(ordinal))
		writeFile(t, filepath.Join(base, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
		classes := filepath.Join(base, "force-app", "main", "default", "classes")
		// Unit 1's helper has one statement, unit 2's has two.
		helper := "public class SharedHelper {\n    public static Integer run() {\n        return 1;\n    }\n}"
		if ordinal == 2 {
			helper = "public class SharedHelper {\n    public static Integer run() {\n        Integer n = 2;\n        return n;\n    }\n}"
		}
		writeFile(t, filepath.Join(classes, "SharedHelper.cls"), helper)
		writeFile(t, filepath.Join(classes, "SharedHelperTest.cls"), fmt.Sprintf("@IsTest private class SharedHelperTest {\n    @IsTest static void check() { System.assertEquals(%d, SharedHelper.run()); }\n}", ordinal))
		for _, name := range []string{"SharedHelper", "SharedHelperTest"} {
			writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, s)
	options := Options{NoDiskCache: true, Parallelism: 1, CodeCoverage: true}
	run := testreport.NativePerTestRows(RunCasesContext(context.Background(), index, options, Discover(index, options)))
	if got := run.Summary(); got.Passed != 2 {
		t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
	}
	checked := 0
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			unit := filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(c.SelectedSourceFile)))))
			for _, row := range c.Coverage {
				if row.Name != "SharedHelper" {
					continue
				}
				checked++
				if !strings.HasPrefix(row.File, unit+string(filepath.Separator)) {
					t.Fatalf("%s row file %s, want under %s", c.SelectedSourceFile, row.File, unit)
				}
				want := 2
				if strings.HasSuffix(unit, "2") {
					want = 3
				}
				if row.NumLinesCovered != want {
					t.Fatalf("%s covered %v, want %d lines", unit, row.CoveredLines, want)
				}
			}
		}
	}
	if checked != 2 {
		t.Fatalf("SharedHelper rows %d", checked)
	}
}
