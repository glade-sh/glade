package apextest

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// This owned API 67 payload passes unchanged on Salesforce. Exercise the
// project runner, which installs generated platform declarations alongside
// the native Formula implementation.
func TestRunFormulaBuilderProjectDispatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC4BuilderProof.cls"), `@IsTest
private class GladeC4BuilderProof {
    @IsTest static void changedContextChangesEvaluation() {
        FormulaEval.FormulaBuilder builder = Formula.builder();
        builder = builder.withType(Account.class);
        builder = builder.withReturnType(FormulaEval.FormulaReturnType.STRING);
        builder = builder.withFormula('Name');
        FormulaEval.FormulaInstance expression = builder.build();
        System.assertNotEquals(null, expression);
        Account record = new Account(Name = 'First');
        System.assertEquals('First', expression.evaluate(record));
        record.Name = 'Second';
        System.assertEquals('Second', expression.evaluate(record));
        System.assertEquals(new Set<String>{'Name'}, expression.getReferencedFields());
    }
}
`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/GladeC4BuilderProof.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>
`)
	run := Run(loadTestIndex(t, root), Options{})
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		data, _ := json.Marshal(run)
		t.Fatalf("project Formula builder dispatch failed: %s", data)
	}
}
