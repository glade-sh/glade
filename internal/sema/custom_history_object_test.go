package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeCustomHistoryObjectFromEnabledMetadata(t *testing.T) {
	for _, api := range []string{"62.0", "64.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			index := customHistoryObjectTestIndex(t, api, "<enableHistory>true</enableHistory>", `
@IsTest private class ParcelHistoryProof {
 @IsTest static void observed() {
  TrackedParcel__c parent = new TrackedParcel__c(Name='Parcel');
  insert parent;
  TrackedParcel__History entry = new TrackedParcel__History(ParentId=parent.Id,Field='Stage__c');
  SObject recordValue = entry;
  List<TrackedParcel__History> entries = new List<TrackedParcel__History>{entry};
  List<SObject> recordValues = entries;
  insert entry;
  List<TrackedParcel__History> queried = [
   SELECT Id,IsDeleted,ParentId,Field,DataType,OldValue,NewValue,CreatedById,CreatedDate
   FROM TrackedParcel__History WHERE ParentId=:parent.Id
  ];
  Object previous = entry.OldValue;
  Object replacement = entry.NewValue;
  String changedField = entry.Field;
  String dataType = entry.DataType;
 }
}`)
			if result := Analyze(index); result.HasErrors() {
				t.Fatalf("native accepts generated history constructor, SObject assignment and test insert: %v", result.Diagnostics)
			}
			model := buildSemaTypeMemberState(prepareAnalysisIndex(index), nil).view()
			history, found := model.lookup(normalizeName("TrackedParcel__History"))
			if !found || !history.sobject {
				t.Fatalf("enabled history metadata did not produce an SObject: %#v", history)
			}
			for expression, want := range map[string]string{
				"entry.OldValue": "Object",
				"entry.NewValue": "Object",
				"entry.Field":    "String",
				"entry.DataType": "String",
				"entry.ParentId": "Id",
				"entry.Parent":   "TrackedParcel__c",
			} {
				got := inferSemaArgTypeWithModel(expression, map[string]string{"entry": "TrackedParcel__History"}, model)
				if got != want {
					t.Errorf("%s type=%q, want %q", expression, got, want)
				}
			}
		})
	}
}

func TestAnalyzeCustomHistoryObjectRequiresEnabledParent(t *testing.T) {
	for _, tc := range []struct{ name, flag string }{
		{"disabled", "<enableHistory>false</enableHistory>"},
		{"omitted", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			index := customHistoryObjectTestIndex(t, "67.0", tc.flag, `
public class ParcelHistoryProof {
 public static SObject ordinary() { return new TrackedParcel__c(Name='Parcel'); }
}`)
			if result := Analyze(index); result.HasErrors() {
				t.Fatalf("ordinary custom object control: %v", result.Diagnostics)
			}
			model := buildSemaTypeMemberState(prepareAnalysisIndex(index), nil).view()
			for _, name := range []string{"TrackedParcel__History", "UnknownParcel__History"} {
				if history, found := model.lookup(normalizeName(name)); found {
					t.Errorf("history generated without enabled parent metadata: %s: %#v", name, history)
				}
				result := AnalyzeAnonymous(index, name+" entry=new "+name+"();", "67.0")
				if !result.HasErrors() {
					t.Errorf("unavailable history type accepted: %s", name)
				}
			}
		})
	}
}

func TestAnalyzeCustomHistoryObjectPreservesLocalRedeclarationControl(t *testing.T) {
	index := customHistoryObjectTestIndex(t, "67.0", "<enableHistory>true</enableHistory>", `
public class ParcelHistoryProof {
 public static void ordinary() {
  Integer quantity=1;
  Integer quantity=2;
 }
}`)
	result := Analyze(index)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "GLADESEMA014" && strings.Contains(diagnostic.Message, "quantity") {
			return
		}
	}
	t.Fatalf("ordinary duplicate local must remain rejected: %v", result.Diagnostics)
}

func customHistoryObjectTestIndex(t *testing.T, api, historyFlag, source string) typesys.Index {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"force-app/main/default/classes", "force-app/main/default/objects/TrackedParcel__c/fields"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
	classPath := filepath.Join(root, "force-app/main/default/classes/ParcelHistoryProof.cls")
	writeSemaFile(t, classPath, source)
	writeSemaFile(t, classPath+"-meta.xml", `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	objectPath := filepath.Join(root, "force-app/main/default/objects/TrackedParcel__c/TrackedParcel__c.object-meta.xml")
	writeSemaFile(t, objectPath, `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><deploymentStatus>Deployed</deploymentStatus><label>Tracked Parcel</label><pluralLabel>Tracked Parcels</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><sharingModel>ReadWrite</sharingModel>`+historyFlag+`</CustomObject>`)
	trackingFlag := ""
	if historyFlag == "<enableHistory>true</enableHistory>" {
		trackingFlag = "<trackHistory>true</trackHistory>"
	}
	writeSemaFile(t, filepath.Join(filepath.Dir(objectPath), "fields/Stage__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Stage__c</fullName><label>Stage</label><type>Text</type><length>80</length>`+trackingFlag+`</CustomField>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	s, err := schema.LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	index := typesys.Build(p, s)
	if index.HasErrors() {
		t.Fatalf("synthetic metadata/type-index setup: %v", index.Diagnostics)
	}
	return index
}
