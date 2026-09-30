package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact three API63 hash assertions admitted by Salesforce SF155.
func TestTypeTokenHashConsistencyAPI63Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTypeHash63Proof.cls"), "@IsTest private class GladeTypeHash63Proof {\n    @IsTest static void standardEqualTokenHashes() {\n        Type lower = Type.forName('account');\n        Type upper = Type.forName('ACCOUNT');\n        Type literal = Account.class;\n        System.assertEquals(literal, lower);\n        System.assertEquals(literal, upper);\n        System.assertEquals(literal.hashCode(), lower.hashCode());\n        System.assertEquals(literal.hashCode(), upper.hashCode());\n    }\n    @IsTest static void customEqualTokenHashes() {\n        Type lower = Type.forName('gladehashsource63');\n        Type upper = Type.forName('GLADEHASHSOURCE63');\n        Type literal = GladeHashSource63.class;\n        System.assertEquals(literal, lower);\n        System.assertEquals(literal, upper);\n        System.assertEquals(literal.hashCode(), lower.hashCode());\n        System.assertEquals(literal.hashCode(), upper.hashCode());\n    }\n    @IsTest static void distinctTokenIdentityControls() {\n        System.assertNotEquals(Account.class, Contact.class);\n        System.assertNotEquals(Type.forName('account'), GladeHashSource63.class);\n        System.assertNotEquals(null, Type.forName('gladehashsource63'));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTypeHash63Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeHashSource63.cls"), "@IsTest public class GladeHashSource63 {}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeHashSource63.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"63.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("report: %s", data)
	if got := run.Summary(); got.Total != 3 || got.Passed != 3 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		t.Fatalf("type hash proof: %s", data)
	}
}
