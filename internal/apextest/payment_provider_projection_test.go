package apextest

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
)

// Native rows: testdata/conformance/csnpe_capture/org.tsv PG_ROW and PG_MISSING.
func TestPaymentProviderMetadataQueryable(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeFile(t, filepath.Join(root, "force-app/main/default/paymentGatewayProviders/ProbeGateway.paymentGatewayProvider-meta.xml"), `<PaymentGatewayProvider xmlns="http://soap.sforce.com/2006/04/metadata"><apexAdapter>ProbeAdapter</apexAdapter><comments>synthetic probe</comments><idempotencySupported>Yes</idempotencySupported><masterLabel>Probe Gateway</masterLabel></PaymentGatewayProvider>`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/ProbeAdapter.cls"), `global class ProbeAdapter implements commercepayments.PaymentGatewayAdapter { global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) { return null; } }`)
			path := filepath.Join(root, "force-app/main/default/classes/ProviderSelectionTest.cls")
			writeFile(t, path, `@IsTest private class ProviderSelectionTest { @IsTest static void findsProvider() {
List<PaymentGatewayProvider> rows = [SELECT DeveloperName, MasterLabel, Comments, IdempotencySupported, NamespacePrefix, ApexAdapterId FROM PaymentGatewayProvider WHERE DeveloperName='ProbeGateway'];
System.assertEquals(1,rows.size());
System.assertEquals('ProbeGateway',rows[0].DeveloperName);
System.assertEquals('Probe Gateway',rows[0].MasterLabel);
System.assertEquals('synthetic probe',rows[0].Comments);
System.assertEquals('Yes',rows[0].IdempotencySupported);
System.assertEquals(null,rows[0].NamespacePrefix);
System.assertNotEquals(null,rows[0].ApexAdapterId);
System.assertEquals([SELECT Id FROM ApexClass WHERE Name='ProbeAdapter' LIMIT 1].Id,rows[0].ApexAdapterId);
System.assertEquals(0,[SELECT count() FROM PaymentGatewayProvider WHERE DeveloperName='ProbeMissing']);
} }`)
			writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
			run := Run(loadTestIndex(t, root), Options{SelectedClasses: []string{"ProviderSelectionTest"}, NoDiskCache: true})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				data, _ := json.Marshal(run)
				t.Fatalf("provider query: %s", data)
			}
		})
	}
}
