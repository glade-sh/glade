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

func TestRuntimeSameNamedTestClassesKeepMetadataContext(t *testing.T) {
	root := t.TempDir()
	for ordinal := 1; ordinal <= 2; ordinal++ {
		base := filepath.Join(root, fmt.Sprint(ordinal))
		writeFile(t, filepath.Join(base, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
		metadata := filepath.Join(base, "force-app", "main", "default")
		writeFile(t, filepath.Join(metadata, "objects", "OwnedMarkerConfig__mdt", "OwnedMarkerConfig__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Owned Marker Config</label><pluralLabel>Owned Marker Configs</pluralLabel><visibility>Public</visibility></CustomObject>`)
		writeFile(t, filepath.Join(metadata, "objects", "OwnedMarkerConfig__mdt", "fields", "Token__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Token__c</fullName><label>Token</label><type>Text</type><length>100</length><required>false</required></CustomField>`)
		for developer, value := range map[string]string{"Marker": fmt.Sprintf("owned-marker-%d", ordinal), "Value": fmt.Sprintf("owned-value-%d", ordinal)} {
			writeFile(t, filepath.Join(metadata, "customMetadata", "OwnedMarkerConfig."+developer+".md-meta.xml"), fmt.Sprintf(`<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>%s</label><protected>false</protected><values><field>Token__c</field><value xsi:type="xsd:string">%s</value></values></CustomMetadata>`, developer, value))
		}
		helper := fmt.Sprintf(`public class UnitMarkerService {
 private static final String marker='owned-marker-%d';
 public class MarkerException extends Exception {}
 public static String read(String developer) {
  List<OwnedMarkerConfig__mdt> rows=[SELECT Token__c FROM OwnedMarkerConfig__mdt WHERE DeveloperName=:developer];
  if(rows.size()==0){throw new MarkerException('missing');}
  if(rows[0].Token__c==marker){throw new MarkerException('marker');}
  return rows[0].Token__c;
 }
}`, ordinal)
		name := "OwnedMarkerUnitTest"
		check := fmt.Sprintf(`@IsTest private class %s { @IsTest static void check(){
 System.assertEquals('owned-value-%d',UnitMarkerService.read('Value'));
 Boolean rejected=false;
 try {UnitMarkerService.read('Marker');}catch(UnitMarkerService.MarkerException expected){rejected=true;}
 System.assertEquals(true,rejected);
} }`, name, ordinal)
		for className, body := range map[string]string{"UnitMarkerService": helper, name: check} {
			classes := filepath.Join(metadata, "classes")
			writeFile(t, filepath.Join(classes, className+".cls"), body)
			writeFile(t, filepath.Join(classes, className+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
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
	options := Options{NoDiskCache: true, Parallelism: 1}
	cases := Discover(index, options)
	if len(cases) != 2 {
		t.Fatalf("discovered %d cases, want 2", len(cases))
	}
	for i, selected := range cases {
		single := RunCasesContext(context.Background(), index, options, []TestCase{selected})
		if got := single.Summary(); got.Total != 1 || got.Passed != 1 {
			t.Fatalf("single context %d: %#v %q", i, got, firstRunProblem(single))
		}
	}
	run := RunCasesContext(context.Background(), index, options, cases)
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
	}
}
