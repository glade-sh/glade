package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact four API63 Type identity assertions admitted by Salesforce SF154.
func TestTypeTokenEqualityAPI63Proof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTypeEquality63Proof.cls"), "@IsTest private class GladeTypeEquality63Proof {\n    @IsTest static void standardTokenCaseEquality() {\n        Type lower = Type.forName('account');\n        Type upper = Type.forName('ACCOUNT');\n        Type literal = Account.class;\n        System.assertNotEquals(null, lower);\n        System.assertNotEquals(null, upper);\n        System.assertEquals('Account', lower.getName());\n        System.assertEquals(true, lower == literal);\n        System.assertEquals(true, literal == upper);\n        System.assertEquals(true, lower.equals(literal));\n        Assert.areEqual(lower, literal, 'Same Account type from a lowercased name');\n    }\n    @IsTest static void customTokenCaseEquality() {\n        Type lower = Type.forName('gladetypesource63');\n        Type upper = Type.forName('GLADETYPESOURCE63');\n        Type literal = GladeTypeSource63.class;\n        System.assertNotEquals(null, lower);\n        System.assertNotEquals(null, upper);\n        System.assertEquals(true, lower == literal);\n        System.assertEquals(true, literal == upper);\n        System.assertEquals(true, lower.equals(literal));\n        Assert.areEqual(lower, literal, 'Same owned Apex class type');\n    }\n    @IsTest static void distinctAndNullControls() {\n        Type accountType = Type.forName('account');\n        System.assertEquals(false, accountType == Contact.class);\n        System.assertEquals(false, accountType.equals(Contact.class));\n        System.assertNotEquals(accountType, GladeTypeSource63.class);\n        System.assertEquals(false, accountType.equals(null));\n        System.assertEquals(null, Type.forName('GladeMissingType63'));\n    }\n    @IsTest static void typeKeysShareIdentity() {\n        Type lower = Type.forName('account');\n        Type literal = Account.class;\n        Set<Type> unique = new Set<Type>{lower, literal, Type.forName('ACCOUNT')};\n        System.assertEquals(1, unique.size());\n        Map<Type, String> values = new Map<Type, String>{lower => 'first'};\n        System.assertEquals('first', values.get(literal));\n        values.put(literal, 'second');\n        System.assertEquals(1, values.size());\n        System.assertEquals('second', values.get(lower));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTypeEquality63Proof.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTypeSource63.cls"), "@IsTest public class GladeTypeSource63 {}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeTypeSource63.cls-meta.xml"), "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>63.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\": \"63.0\", \"namespace\": \"\", \"packageDirectories\": [{\"path\": \"force-app\", \"default\": true}]}")
	run := Run(loadTestIndex(t, root), Options{NoDiskCache: true})
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("report: %s", data)
	if got := run.Summary(); got.Total != 4 || got.Passed != 4 || got.Failed != 0 || got.Errors != 0 || got.Skipped != 0 {
		t.Fatalf("type equality proof: %s", data)
	}
}
