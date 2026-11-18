package sema

import (
	"strings"
	"testing"
)

func TestStandardControllerGetRecordAllowsIdButRejectsUnknownFields(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"WidgetExt.cls": `
public class WidgetExt {
  public Id recordId;
  public WidgetExt(ApexPages.StandardController ctl) {
    recordId = (ctl.getRecord()).Id;
  }
  public static Object bad(ApexPages.StandardController ctl) { return (ctl.getRecord()).NoSuchThing__c; }
}
`,
	})
	var vf []string
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA_VF001" {
			vf = append(vf, diag.Message)
		}
	}
	joined := strings.Join(vf, "\n")
	if strings.Contains(joined, "Variable does not exist: Id") {
		t.Fatalf("getRecord().Id was rejected: %s", joined)
	}
	if !strings.Contains(joined, "NoSuchThing__c") {
		t.Fatalf("unknown getRecord() field was accepted: %#v", result.Diagnostics)
	}
}
