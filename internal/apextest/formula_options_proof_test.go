package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeC4TemplateProof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC4TemplateProof.cls"), "@IsTest\nprivate class GladeC4TemplateProof {\n    @IsTest static void templateUsesChangedContext() {\n        FormulaEval.FormulaInstance expression = Formula.builder().withType(Account.class)\n            .withReturnType(FormulaEval.FormulaReturnType.STRING)\n            .withFormula('Company: {!Name}').parseAsTemplate(true).build();\n        System.assertEquals('Company: First', expression.evaluate(new Account(Name = 'First')));\n        System.assertEquals('Company: Second', expression.evaluate(new Account(Name = 'Second')));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC4TemplateProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c4-formula-template\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}

// Exact owned Salesforce-admitted source and component metadata.
func TestRunGladeC4NullProof(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC4NullProof.cls"), "@IsTest\nprivate class GladeC4NullProof {\n    @IsTest static void nullModeChangesResult() {\n        FormulaEval.FormulaInstance zero = Formula.builder().withType(Account.class)\n            .withReturnType(FormulaEval.FormulaReturnType.DECIMAL)\n            .withFormula('AnnualRevenue + 1').treatNumericNullAsZero(true).build();\n        FormulaEval.FormulaInstance blank = Formula.builder().withType(Account.class)\n            .withReturnType(FormulaEval.FormulaReturnType.DECIMAL)\n            .withFormula('AnnualRevenue + 1').treatNumericNullAsZero(false).build();\n        System.assertEquals(1, zero.evaluate(new Account()));\n        System.assertEquals(null, blank.evaluate(new Account()));\n        System.assertEquals(3, zero.evaluate(new Account(AnnualRevenue = 2)));\n        System.assertEquals(3, blank.evaluate(new Account(AnnualRevenue = 2)));\n    }\n}\n")
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC4NullProof.cls-meta.xml"), "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
	writeFile(t, filepath.Join(root, "sfdx-project.json"), "{\"packageDirectories\":[{\"path\":\"force-app\",\"default\":true}],\"name\":\"c4-formula-null-option\",\"namespace\":\"\",\"sfdcLoginUrl\":\"https://login.salesforce.com\",\"sourceApiVersion\":\"67.0\"}")
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("admitted contract failed: %s", data)
	}
}
