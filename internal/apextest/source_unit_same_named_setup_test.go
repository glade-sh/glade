package apextest

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Two same-named test classes from different source units each run only
// their own @TestSetup: each test sees exactly its own unit's marker row.
func TestRuntimeSameNamedTestClassesRunOwnTestSetup(t *testing.T) {
	root := t.TempDir()
	for ordinal := 1; ordinal <= 2; ordinal++ {
		base := filepath.Join(root, fmt.Sprint(ordinal))
		writeFile(t, filepath.Join(base, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
		classes := filepath.Join(base, "force-app", "main", "default", "classes")
		body := fmt.Sprintf(`@IsTest private class OwnedSetupUnitTest {
 @TestSetup static void seed() { insert new Account(Name = 'unit-marker-%d'); }
 @IsTest static void check() {
  System.assertEquals(1, [SELECT count() FROM Account WHERE Name LIKE 'unit-marker-%%']);
  System.assertEquals(1, [SELECT count() FROM Account WHERE Name = 'unit-marker-%d']);
 }
}`, ordinal, ordinal)
		writeFile(t, filepath.Join(classes, "OwnedSetupUnitTest.cls"), body)
		writeFile(t, filepath.Join(classes, "OwnedSetupUnitTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
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
	cases := Discover(index, options)
	if len(cases) != 2 {
		t.Fatalf("discovered %d cases, want 2", len(cases))
	}
	run := RunCasesContext(context.Background(), index, options, cases)
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
	}
}
