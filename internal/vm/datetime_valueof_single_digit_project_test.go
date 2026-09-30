package vm_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Salesforce admits API62 Datetime.valueOf padding of each individual local
// date and clock component.
func TestDatetimeValueOfSingleDigitClockAPI62Project(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"sfdx-project.json": `{"namespace":"","sourceApiVersion":"40.0","packageDirectories":[{"path":"force-app","default":true}]}`,
		"force-app/main/default/classes/GladeVolunteersDatetime62Proof.cls": `@IsTest private class GladeVolunteersDatetime62Proof {
 @IsTest static void originalSingleDigitRangeMatchesPadded() {
  String strStartDateTime = '2010-01-01 1:1:1';
  String strEndDateTime = '2050-01-01 1:1:1';
  DateTime dtStart = datetime.valueOf(strStartDateTime);
  DateTime dtEnd = datetime.valueOf(strEndDateTime);
  System.assertEquals(datetime.valueOf('2010-01-01 01:01:01'), dtStart);
  System.assertEquals(datetime.valueOf('2050-01-01 01:01:01'), dtEnd);
  System.assert(dtStart < dtEnd);
 }
 @IsTest static void individualClockComponentsMatchPadded() {
  DateTime padded = datetime.valueOf('2010-01-01 01:01:01');
  System.assertEquals(datetime.valueOf('2006-05-04 03:02:01'), datetime.valueOf('2006-5-4 3:2:1'));
  System.assertEquals(datetime.valueOf('2006-05-04 03:02:01'), datetime.valueOf('2006-5-04 03:02:01'));
  System.assertEquals(datetime.valueOf('2006-05-04 03:02:01'), datetime.valueOf('2006-05-4 03:02:01'));
  System.assertEquals(padded, datetime.valueOf('2010-01-01 1:01:01'));
  System.assertEquals(padded, datetime.valueOf('2010-01-01 01:1:01'));
  System.assertEquals(padded, datetime.valueOf('2010-01-01 01:01:1'));
 }
}`,
		"force-app/main/default/classes/GladeVolunteersDatetime62Proof.cls-meta.xml": `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`,
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
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
	r := apextest.Run(typesys.Build(p, s), apextest.Options{NoDiskCache: true})
	if got := r.Summary(); got.Total != 2 || got.Passed != 2 || got.Errors != 0 {
		t.Fatalf("single-digit Datetime.valueOf: %+v; %+v", got, r)
	}
}
