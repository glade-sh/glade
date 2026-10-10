package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact Salesforce-admitted source and metadata.
func TestRunFormulaFollowupFieldReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC4ReferencesProof.cls"), "@IsTest private class GladeC4ReferencesProof {\n @IsTest static void templateReferencesOnlyMergedFields() {\n  FormulaEval.FormulaInstance f = Formula.builder().withType(Account.class).withReturnType(FormulaEval.FormulaReturnType.STRING).withFormula('Company: {!name}').parseAsTemplate(true).build();\n  System.assertEquals(new Set<String>{'name'}, f.getReferencedFields());\n  System.assertEquals('Company: Owned', f.evaluate(new Account(Name='Owned')));\n }\n @IsTest static void quotedWordsAreNotFieldReferences() {\n  FormulaEval.FormulaInstance f = Formula.builder().withType(Account.class).withReturnType(FormulaEval.FormulaReturnType.STRING).withFormula('name & \" Website\"').build();\n  System.assertEquals(new Set<String>{'name'}, f.getReferencedFields());\n  System.assertEquals('Owned Website', f.evaluate(new Account(Name='Owned')));\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC4ReferencesProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 2 || got.Passed != 2 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact Salesforce-admitted source and metadata.
func TestRunFormulaFollowupNestedNumericNull(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC4NestedNullProof.cls"), "@IsTest private class GladeC4NestedNullProof {\n @IsTest static void nestedFunctionPreservesNumericNullMode() {\n  FormulaEval.FormulaInstance zero = Formula.builder().withType(Account.class).withReturnType(FormulaEval.FormulaReturnType.DECIMAL).withFormula('IF(true, AnnualRevenue + 1, 0)').treatNumericNullAsZero(true).build();\n  FormulaEval.FormulaInstance blank = Formula.builder().withType(Account.class).withReturnType(FormulaEval.FormulaReturnType.DECIMAL).withFormula('IF(true, AnnualRevenue + 1, 0)').treatNumericNullAsZero(false).build();\n  System.assertEquals(1, zero.evaluate(new Account()));\n  System.assertEquals(null, blank.evaluate(new Account()));\n  System.assertEquals(3, zero.evaluate(new Account(AnnualRevenue=2)));\n  System.assertEquals(3, blank.evaluate(new Account(AnnualRevenue=2)));\n }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC4NestedNullProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"sourceApiVersion\":\"67.0\",\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}]}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}
