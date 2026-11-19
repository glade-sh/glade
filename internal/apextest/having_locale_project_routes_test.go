package apextest

import (
	"path/filepath"
	"testing"
)

// Local runner twin for locale-preserving HAVING. The comparator is backed by
// native text-range rows; no native HAVING transport observation is claimed.
func TestHavingTextLocaleProjectRunner(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	classes := filepath.Join(root, "force-app/main/default/classes")
	for _, api := range []string{"62.0", "67.0"} {
		name := "HavingLocaleTest" + api[:2]
		writeFile(t, filepath.Join(classes, name+".cls"), `@IsTest private class `+name+` {
   @IsTest static void groupedText() {
    insert new Account(Name='_');
    List<Account> w = [SELECT Id FROM Account WHERE Name > '9'];
    List<AggregateResult> g = [SELECT Name, COUNT(Id) c FROM Account GROUP BY Name HAVING Name > '9'];
    System.assertEquals(0,w.size()); System.assertEquals(w.size(),g.size());
    System.assertEquals(w.size(),Database.query('SELECT Name, COUNT(Id) c FROM Account GROUP BY Name HAVING Name > \'9\'').size());
   }
  }`)
		writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	}
	index := loadTestIndex(t, root)
	run := Run(index, Options{SelectedClasses: []string{"HavingLocaleTest62", "HavingLocaleTest67"}, NoDiskCache: true, Parallelism: 1})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("%+v: %s", got, firstRunProblem(run))
	}
}
