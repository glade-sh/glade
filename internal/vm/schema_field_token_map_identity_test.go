package vm_test

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Synthetic string keys deliberately differ from the fields whose tokens they
// store. A token-valued ordinary map must not acquire describe-map aliases.
// Native API 62/67/68 anonymous and @IsTest observations are exported by
// TestSchemaFieldTokenMapOrgConformance. These repetitions exercise isolation
// and map order beyond the exported single-row assertions.
const fieldTokenMapSetup = `
Schema.SObjectField contactName = Schema.SObjectType.Contact.fields.getMap().get('Name');
Schema.SObjectField accountName = Schema.SObjectType.Account.fields.getMap().get('Name');
Map<String,%[1]s> fields = new Map<String,%[1]s>{
    'slot__c' => contactName,
    'probe__slot__c' => accountName,
    'probe__other__c' => accountName,
    'createddate' => Schema.SObjectType.Contact.fields.getMap().get('CreatedDate')
};
`

const fieldTokenMapMissingKey = `
System.assertEquals(contactName, fields.get('slot__c'));
System.assertEquals(accountName, fields.get('probe__slot__c'));
System.assertNotEquals(contactName, accountName);
System.assertEquals(null, fields.get('absent__slot__c'), 'ordinary map missing key');
System.assertEquals(false, fields.containsKey('absent__slot__c'), 'ordinary map missing membership');
System.assertEquals(null, fields.get('SLOT__C'), 'ordinary map case boundary');
System.assertEquals(false, fields.containsKey('SLOT__C'));
System.assertEquals(4, fields.size());
`

const fieldTokenMapSelection = `
String requested = 'SLoT__C';
String preferred = ('absent__' + requested).toLowerCase();
Object selected;
if (fields.containsKey(preferred)) {
    selected = fields.get(preferred);
} else {
    selected = fields.get(requested.toLowerCase());
}
System.assertNotEquals(selected, fields.get('probe__slot__c'), 'different object tokens after selection');
System.assertEquals(contactName, selected, 'fallback selects exact unqualified entry');
preferred = ('probe__' + requested).toLowerCase();
System.assertEquals(true, fields.containsKey(preferred));
System.assertEquals(accountName, fields.get(preferred), 'explicit existing namespace key');
Map<String,Schema.SObjectField> described = Schema.SObjectType.Contact.fields.getMap();
System.assertEquals(contactName, described.get('NAME'), 'describe map retains case aliases');
System.assertEquals(null, described.get('MissingField__c'), 'describe map missing field');
System.assertEquals(false, described.containsKey('MissingField__c'));
System.assertEquals(Account.Name, accountName);
System.assertEquals(Contact.Name, contactName);
System.assertEquals('Contact', contactName.getDescribe().getSObjectType().getDescribe().getName());
System.assertEquals('Account', accountName.getDescribe().getSObjectType().getDescribe().getName());
`

func TestSchemaFieldTokenMapLookupRepeatedAnonymous(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, valueType := range []string{"Schema.SObjectField", "Object"} {
			for _, scenario := range []struct{ name, body string }{
				{"missingKey", fieldTokenMapMissingKey},
				{"selection", fieldTokenMapSelection},
			} {
				t.Run(api+"/"+valueType+"/"+scenario.name, func(t *testing.T) {
					program, err := vm.CompileAnonymousWithOptions(fmt.Sprintf(fieldTokenMapSetup, valueType)+scenario.body, vm.CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					shuffle := rand.New(rand.NewSource(123))
					failures := 0
					var firstError error
					for iteration := 0; iteration < 200; iteration++ {
						// Fresh schema maps and VM caches each time; vary insertion order
						// as well as the independent Go map iteration seed.
						org := storage.NewOrgState()
						for _, objectIndex := range shuffle.Perm(2) {
							name := []string{"Account", "Contact"}[objectIndex]
							fields := make(map[string]storage.Field)
							for _, fieldIndex := range shuffle.Perm(2) {
								fieldName := []string{"Name", "CreatedDate"}[fieldIndex]
								fieldType := storage.FieldString
								if fieldName == "CreatedDate" {
									fieldType = storage.FieldDateTime
								}
								fields[fieldName] = storage.Field{APIName: fieldName, Type: fieldType}
							}
							org.Objects[name] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: name, Fields: fields}}
						}
						machine := vm.New(nil)
						machine.SetOrg(&org)
						if _, err := machine.Execute(program); err != nil {
							failures++
							if firstError == nil {
								firstError = err
							}
						}
					}
					if failures != 0 {
						t.Fatalf("%d/200 fresh executions failed; first: %v", failures, firstError)
					}
				})
			}
		}
	}
}

func TestSchemaFieldTokenMapLookupRepeatedIsTest(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			var source strings.Builder
			source.WriteString("@IsTest private class FieldTokenMapProbe {\n")
			for typeIndex, valueType := range []string{"Schema.SObjectField", "Object"} {
				for _, scenario := range []struct{ name, body string }{
					{"missingKey", fieldTokenMapMissingKey},
					{"selection", fieldTokenMapSelection},
				} {
					// Each named method receives the runner's own org/VM clone.
					for iteration := 0; iteration < 50; iteration++ {
						fmt.Fprintf(&source, "@IsTest static void %s%d_%d(){\n%s%s\n}\n", scenario.name, typeIndex, iteration, fmt.Sprintf(fieldTokenMapSetup, valueType), scenario.body)
					}
				}
			}
			source.WriteString("}\n")
			files := map[string]string{
				"sfdx-project.json": fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api),
				"force-app/main/default/classes/FieldTokenMapProbe.cls":          source.String(),
				"force-app/main/default/classes/FieldTokenMapProbe.cls-meta.xml": fmt.Sprintf(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>%s</apiVersion><status>Active</status></ApexClass>`, api),
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
			index := typesys.Build(p, schema.Schema{})
			run := apextest.Run(index, apextest.Options{NoDiskCache: true, Parallelism: 1})
			count, failures := 0, 0
			firstProblem := ""
			for _, suite := range run.Suites {
				for _, tc := range suite.Cases {
					count++
					if tc.Status != testreport.StatusPass {
						failures++
						if firstProblem == "" {
							firstProblem = fmt.Sprintf("%s: %s: %+v", tc.MethodName, tc.Status, tc.Problem)
						}
					}
				}
			}
			if count != 200 || failures != 0 {
				t.Fatalf("%d/200 named methods selected, %d failed; first: %s", count, failures, firstProblem)
			}
		})
	}
}
