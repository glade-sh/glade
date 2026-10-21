package apextest

import (
	"fmt"
	"path/filepath"
	"testing"
)

// Explicit superclass initialization stays visible to an inherited method
// invoked from a trigger when the child declares a differently typed field.
func TestProtectedRedefinedFieldThroughNamedTriggerRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			base := filepath.Join(root, "force-app/main/default")
			classes := map[string]string{
				"SyncConfig": `public virtual class SyncConfig {
  public List<String> someList = new List<String>{'ready'};
}`,
				"DerivedConfig": `public class DerivedConfig extends SyncConfig {}`,
				"TriggerDomainBase": `public virtual class TriggerDomainBase {
  protected SyncConfig syncConfig;
  public void inspectConfig() {
    for (String value : syncConfig.someList) {
      System.assertEquals('ready', value);
    }
  }
}`,
				"TriggerDomainChild": `public class TriggerDomainChild extends TriggerDomainBase {
  protected DerivedConfig syncConfig;
  public TriggerDomainChild() {
    super.syncConfig = new SyncConfig();
    this.syncConfig = new DerivedConfig();
  }
  public void beforeInsert(List<Account> rows) {
    super.inspectConfig();
  }
}`,
				"ProtectedFieldTriggerTest": `@IsTest private class ProtectedFieldTriggerTest {
  @IsTest static void namedInsertRoute() {
    Account row = new Account(Name='Synthetic');
    insert row;
    System.assertNotEquals(null, row.Id);
  }
}`,
			}
			for name, source := range classes {
				writeFile(t, filepath.Join(base, "classes", name+".cls"), source)
				writeFile(t, filepath.Join(base, "classes", name+".cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			}
			writeFile(t, filepath.Join(base, "triggers/ProtectedFieldTrigger.trigger"), `trigger ProtectedFieldTrigger on Account (before insert) {
  TriggerDomainChild handler = new TriggerDomainChild();
  handler.beforeInsert(Trigger.new);
}`)
			writeFile(t, filepath.Join(base, "triggers/ProtectedFieldTrigger.trigger-meta.xml"), `<ApexTrigger xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexTrigger>`)

			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"ProtectedFieldTriggerTest"}, NoDiskCache: true, Parallelism: 1})
			if got := run.Summary(); got.Total != 1 || got.Passed != 1 || got.Failed != 0 || got.Errors != 0 || got.Unsupported != 0 {
				t.Fatalf("protected field trigger route: summary=%+v, problem=%s", got, firstRunProblem(run))
			}
		})
	}
}
