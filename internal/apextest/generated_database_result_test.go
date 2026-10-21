package apextest

import (
	"path/filepath"
	"testing"
)

// dml_transactions R025-R027/R035-R037/R124/R144 own the success, ID,
// empty-error and failed-required-field answers. Exercise them with the full
// linked platform catalog instead of only the anonymous VM's runtime handlers.
func TestRunDatabaseResultsWithLoadedCatalog(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"parcel","sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/fields/Name.field-meta.xml"), `<CustomField><fullName>Name</fullName><label>Name</label><type>Text</type><required>true</required></CustomField>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ReceiptView.cls"), `
public class ReceiptView {
  public Boolean isSuccess(){return false;}
  public List<String> getErrors(){return new List<String>{'owned'};}
  public String getStatusCode(){return 'owned';}
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/ResultCatalogProbe.cls"), `
@IsTest private class ResultCatalogProbe {
  @IsTest static void successfulCreateAndUpdate(){
    Account parent=new Account(Name='Synthetic result');
    Database.SaveResult inserted=Database.insert(parent,false);
    System.assertEquals(true,inserted.isSuccess());
    System.assertEquals(parent.Id,inserted.getId());
    System.assertEquals(0,inserted.getErrors().size());
    parent.Name='Synthetic changed';
    Database.SaveResult updated=Database.update(parent,false);
    System.assertEquals(true,updated.isSuccess());
    System.assertEquals(parent.Id,updated.getId());
    System.assertEquals(0,updated.getErrors().size());
  }
  @IsTest static void failedRequiredFieldUpdate(){
    Account parent=new Account(Name='Synthetic result');insert parent;
    parent.Name=null;
    Database.SaveResult failed=Database.update(parent,false);
    System.assertEquals(false,failed.isSuccess());
    System.assertEquals(parent.Id,failed.getId());
    System.assertEquals(1,failed.getErrors().size());
    Database.Error issue=failed.getErrors()[0];
    System.assertEquals(System.StatusCode.REQUIRED_FIELD_MISSING,issue.getStatusCode());
    System.assertEquals('Name',issue.getFields()[0]);
    System.assert(issue.getMessage().contains('Name'));
    System.assertEquals('Synthetic result',[SELECT Name FROM Account WHERE Id=:parent.Id].Name);
  }
  @IsTest static void projectGetterControl(){
    ReceiptView view=new ReceiptView();
    System.assertEquals(false,view.isSuccess());
    System.assertEquals('owned',view.getErrors()[0]);
    System.assertEquals('owned',view.getStatusCode());
  }
}`)
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 3 || got.Passed != 3 {
		t.Fatalf("summary = %#v; problem = %s", got, firstRunProblem(run))
	}
}
