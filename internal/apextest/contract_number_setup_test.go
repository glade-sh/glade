package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestContractNumberSF232(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"namespace": "", "sourceApiVersion": "65.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeContractNumber65Assertions.cls"), `@IsTest private class GladeContractNumber65Assertions {
 @IsTest static void assertStoredNumbersAndCallerState() {
  Account a=new Account(Name='Owned contract number assertions'); insert a;
  List<Contract> rows=new List<Contract>{new Contract(AccountId=a.Id,StartDate=Date.today(),ContractTerm=1),new Contract(AccountId=a.Id,StartDate=Date.today(),ContractTerm=1)}; insert rows;
  System.assertEquals(null,rows[0].ContractNumber); System.assertEquals(null,rows[1].ContractNumber);
  List<Contract> stored=[SELECT Id,ContractNumber FROM Contract WHERE AccountId=:a.Id]; System.assertEquals(2,stored.size());
  Set<String> numbers=new Set<String>();
  for(Contract c:stored){System.assertNotEquals(null,c.ContractNumber);System.assertEquals(true,Pattern.matches('[0-9]{8}',c.ContractNumber));numbers.add(c.ContractNumber);}
  System.assertEquals(2,numbers.size());
 }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeContractNumber65Assertions.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>65.0</apiVersion><status>Active</status></ApexClass>`)
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		b, _ := json.Marshal(run)
		t.Fatalf("ContractNumber: %s", b)
	}
}
