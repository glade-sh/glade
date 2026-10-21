package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// The ordinary selector contract follows the public ApexTrigger object fields.
// The inherited automatic-property collision is a local regression candidate;
// its exact lexical precedence still requires native rows.
func TestSourceTriggerNameSelector(t *testing.T) {
	runSourceTriggerSelector(t, false)
}

func TestSourceTriggerNameSelectorInheritedPropertyCandidate(t *testing.T) {
	runSourceTriggerSelector(t, true)
}

func runSourceTriggerSelector(t *testing.T, inherited bool) {
	t.Helper()
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, namespace := range []string{"", "parcel"} {
			t.Run(api+"/"+namespace, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"namespace":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api, namespace))
				base := filepath.Join(root, "force-app/main/default")
				classes := map[string]string{
					"SelectorOwner": `public virtual class SelectorOwner {
 public static PredicateHarness Test {get;private set;}
 static {Test=new PredicateHarness();}
 public class PredicateHarness {public String Database='marker';}
}`,
					"MetadataSelector": `public class MetadataSelector extends SelectorOwner {
 public static String selectedName() {return Test.isRunningTest()?'ParcelWitness':'AbsentWitness';}
 public static Map<String,ApexTrigger> byName(Set<String> names) {
  List<ApexTrigger> rows=Database.query('SELECT Id,Name,ApiVersion,Body,BodyCrc,CreatedById,CreatedDate,IsValid,LastModifiedById,LastModifiedDate,LengthWithoutComments,NamespacePrefix,Status,SystemModstamp,TableEnumOrId,UsageAfterDelete,UsageAfterInsert,UsageAfterUndelete,UsageAfterUpdate,UsageBeforeDelete,UsageBeforeInsert,UsageBeforeUpdate,UsageIsBulk FROM ApexTrigger WHERE Name IN :names ORDER BY Name ASC NULLS FIRST');
  Map<String,ApexTrigger> result=new Map<String,ApexTrigger>();
  for(ApexTrigger row:rows) result.put(row.Name,row);
  return result;
 }
}`,
					"MetadataSelectorProbe": `@IsTest private class MetadataSelectorProbe {
 @IsTest static void selectedTrigger() {
  String name=SELECTED_NAME;
  Map<String,ApexTrigger> rows=MetadataSelector.byName(new Set<String>{name,'AbsentWitness'});
  System.assertEquals(1,rows.size(),'project trigger by Name');
  ApexTrigger row=rows.get('ParcelWitness');
  System.assertNotEquals(null,row.Id);
  System.assertEquals('ParcelWitness',row.Name);
  System.assertEquals('Active',row.Status);
  System.assertEquals(true,row.IsValid);
  System.assertEquals('Contact',row.TableEnumOrId);
  System.assertEquals(API_VERSION,row.ApiVersion);
  System.assert(row.Body.contains('trigger ParcelWitness'));
  System.assertEquals('marker',MetadataSelector.Test.Database);
  System.assertEquals(1,[SELECT COUNT() FROM ApexTrigger WHERE Name='ParcelWitness']);
  System.assertEquals(0,[SELECT COUNT() FROM ApexTrigger WHERE Name='AbsentWitness']);
 }
}`,
				}
				selected := "'ParcelWitness'"
				if inherited {
					selected = "MetadataSelector.selectedName()"
				}
				classes["MetadataSelectorProbe"] = strings.ReplaceAll(strings.ReplaceAll(classes["MetadataSelectorProbe"], "SELECTED_NAME", selected), "API_VERSION", api)
				for name, source := range classes {
					path := filepath.Join(base, "classes", name+".cls")
					writeFile(t, path, source)
					writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				}
				writeFile(t, filepath.Join(base, "triggers/ParcelWitness.trigger"), "trigger ParcelWitness on Contact (before insert, after update) {}")
				writeFile(t, filepath.Join(base, "triggers/ParcelWitness.trigger-meta.xml"), "<ApexTrigger><apiVersion>"+api+"</apiVersion><status>Active</status></ApexTrigger>")
				run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"MetadataSelectorProbe"}, NoDiskCache: true})
				if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
					data, _ := json.Marshal(run)
					t.Fatalf("trigger selector: %s", data)
				}
			})
		}
	}
}
