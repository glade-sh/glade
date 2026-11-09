package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSchemaShorthandPathsAndComparisonInitializers(t *testing.T) {
	root := t.TempDir()
	writeShorthandFile(t, filepath.Join(root, "force-app/objects/Widget__c/Widget__c.object-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	writeShorthandFile(t, filepath.Join(root, "force-app/objects/Widget__c/fields/Parent__c.field-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Parent__c</fullName><label>Parent</label><referenceTo>Account</referenceTo><relationshipName>Widgets</relationshipName><type>Lookup</type></CustomField>`)
	writeShorthandFile(t, filepath.Join(root, "force-app/objects/Widget__c/fieldSets/MainList.fieldSet-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<FieldSet xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>MainList</fullName><description>d</description><label>Main</label><displayedFields><field>Name</field><isFieldManaged>false</isFieldManaged><isRequired>false</isRequired></displayedFields></FieldSet>`)
	writeShorthandFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"64.0"}`)
	writeShorthandFile(t, filepath.Join(root, "force-app/classes/Syntax.cls"), `
public class Syntax {
  public static Boolean between(Date a, Date b, Date c) {
    Boolean inside = a < b && c > b;
    return inside;
  }
  public class Item { public Date dateFrom; public String kind; }
  public static Boolean chain(List<Item> items, Date currentDate, Date dateTo) {
    Boolean ok = items.get(0).kind == 'CAR' && items.get(0).dateFrom < currentDate && dateTo > currentDate;
    Boolean ok3 = currentDate < dateTo && dateTo > currentDate;
    return ok && ok3;
  }
  public static String fieldName() {
    return Widget__c.sObjectType.Parent__c.getDescribe().getName();
  }
  public static Integer fieldSetSize() {
    Schema.FieldSet fs = SObjectType.Widget__c.FieldSets.MainList;
    return fs.getFields().size();
  }
  public static String missingField() {
    return Widget__c.sObjectType.NoSuchField__c.getDescribe().getName();
  }
}
`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	result := Analyze(typesys.Build(p, sch))
	var messages []string
	for _, diag := range result.Diagnostics {
		messages = append(messages, diag.Code+" "+diag.Message)
	}
	joined := strings.Join(messages, "\n")
	for _, unwanted := range []string{`declares local "b"`, `local "ok" with`, `declares local "currentDate"`, `"Widget__c.sObjectType.Parent__c.getDescribe"`, `local "fs" with List<Object>`} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("unexpected diagnostic (%s):\n%s", unwanted, joined)
		}
	}
	if !strings.Contains(joined, "NoSuchField__c") {
		t.Fatalf("unknown field through SObjectType shortcut was accepted:\n%s", joined)
	}
}

func writeShorthandFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSemaFile(t, path, contents)
}
