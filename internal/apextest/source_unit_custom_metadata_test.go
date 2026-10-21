package apextest

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestRuntimeSourceUnitCustomMetadataKeepsProjectContext(t *testing.T) {
	root := t.TempDir()
	for i, unit := range []string{"first", "second"} {
		base := filepath.Join(root, unit)
		writeFile(t, filepath.Join(base, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
		metadata := filepath.Join(base, "force-app", "main", "default")
		writeFile(t, filepath.Join(metadata, "objects", "OwnedUnitConfig__mdt", "OwnedUnitConfig__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Owned Unit Config</label><pluralLabel>Owned Unit Configs</pluralLabel><visibility>Public</visibility></CustomObject>`)
		writeFile(t, filepath.Join(metadata, "objects", "OwnedUnitConfig__mdt", "fields", "Ordinal__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Ordinal__c</fullName><label>Ordinal</label><type>Number</type><precision>18</precision><scale>0</scale><required>false</required></CustomField>`)
		writeFile(t, filepath.Join(metadata, "customMetadata", "OwnedUnitConfig.shared.md-meta.xml"), fmt.Sprintf(`<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>Shared</label><protected>false</protected><values><field>Ordinal__c</field><value xsi:type="xsd:double">%d</value></values></CustomMetadata>`, i+1))
		classes := filepath.Join(metadata, "classes")
		helper := `public class UnitMetadataReader { public static List<OwnedUnitConfig__mdt> read(){return [SELECT Ordinal__c FROM OwnedUnitConfig__mdt WHERE DeveloperName='shared'];} }`
		name := fmt.Sprintf("OwnedMetadataContext%dTest", i+1)
		check := fmt.Sprintf(`@IsTest private class %s { @IsTest static void check(){ List<OwnedUnitConfig__mdt> rows=UnitMetadataReader.read();System.assertEquals(1,rows.size());System.assertEquals(%d,rows[0].Ordinal__c); } }`, name, i+1)
		for className, body := range map[string]string{"UnitMetadataReader": helper, name: check} {
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
	run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		t.Fatalf("summary=%#v problem=%q", got, firstRunProblem(run))
	}
}
