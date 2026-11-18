package apextest

import (
	"path/filepath"
	"strings"
	"testing"
)

// Named-runner twins use the same captured ä/z distinction as the SOQL and
// VM fixtures. System.runAs supplies LocaleSidKey independently of language.
func TestLocaleCollationNamedRunAsRoutes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	classes := filepath.Join(root, "force-app/main/default/classes")
	for _, api := range []string{"62.0", "67.0"} {
		name := "LocaleComparison" + api[:2]
		var source strings.Builder
		source.WriteString("@IsTest private class " + name + " {\n")
		for _, tc := range []struct{ method, locale, first string }{{"root", "de_DE", "ä"}, {"tailored", "sv_SE", "z"}, {"fallback", "af_ZA", "z"}} {
			source.WriteString(`@IsTest static void ` + tc.method + `() {
    User u=new User(Alias='locale',Email='locale@example.invalid',LastName='Locale',Username='locale.` + tc.locale + `@example.invalid',ProfileId=UserInfo.getProfileId(),LocaleSidKey='` + tc.locale + `',LanguageLocaleKey='en_US',TimeZoneSidKey='GMT',EmailEncodingKey='UTF-8');
    insert u;
    System.runAs(u) {
     Account a=new Account(Name='ä'); Account z=new Account(Name='z'); insert new List<Account>{a,z};
     Set<Id> ids=new Set<Id>{a.Id,z.Id};
     List<Account> rows=[SELECT Name FROM Account WHERE Id IN :ids ORDER BY Name];
     System.assertEquals('` + tc.first + `',rows[0].Name);
     List<Account> dynamicRows=Database.query('SELECT Name FROM Account WHERE Id IN :ids ORDER BY Name');
     System.assertEquals(rows[0].Name,dynamicRows[0].Name);
     List<AggregateResult> mm=[SELECT MIN(Name) mn FROM Account WHERE Id IN :ids];
     System.assertEquals('` + tc.first + `',mm[0].get('mn'));
    }
   }
`)
		}
		source.WriteString("}\n")
		writeFile(t, filepath.Join(classes, name+".cls"), source.String())
		writeFile(t, filepath.Join(classes, name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	}
	index := loadTestIndex(t, root)
	run := Run(index, Options{SelectedClasses: []string{"LocaleComparison62", "LocaleComparison67"}, NoDiskCache: true, Parallelism: 1})
	if got := run.Summary(); got.Total != 6 || got.Passed != 6 {
		t.Fatalf("%+v: %s", got, firstRunProblem(run))
	}
}
