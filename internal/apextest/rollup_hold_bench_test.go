package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"
)

// Bulk child inserts into one parent with a COUNT and a SUM roll-up, with and
// without an after-trigger on the child. The trigger switches on the frozen
// roll-up read; without it the statement does no extra work. Set
// GLADE_ROLLUP_HOLD_PERF=1 to print the timings (ms per run, sorted).
func rollupBulkInsertMillis(t *testing.T, withTrigger bool) []float64 {
	root := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		writeFile(t, filepath.Join(root, "force-app/main/default", rel), body)
	}
	api := "62.0"
	writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
	write("objects/HbP__c/HbP__c.object-meta.xml", `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Hb P</label><pluralLabel>Hb Ps</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
	write("objects/HbP__c/fields/Cnt__c.field-meta.xml", `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Cnt__c</fullName><label>Cnt</label><summaryForeignKey>HbC__c.P__c</summaryForeignKey><summaryOperation>count</summaryOperation><type>Summary</type></CustomField>`)
	write("objects/HbP__c/fields/Tot__c.field-meta.xml", `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Tot__c</fullName><label>Tot</label><summarizedField>HbC__c.Amt__c</summarizedField><summaryForeignKey>HbC__c.P__c</summaryForeignKey><summaryOperation>sum</summaryOperation><type>Summary</type></CustomField>`)
	write("objects/HbC__c/HbC__c.object-meta.xml", `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Hb C</label><pluralLabel>Hb Cs</pluralLabel><nameField><label>Name</label><type>Text</type></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ControlledByParent</sharingModel></CustomObject>`)
	write("objects/HbC__c/fields/P__c.field-meta.xml", `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>P__c</fullName><label>P</label><referenceTo>HbP__c</referenceTo><relationshipLabel>Hb Cs</relationshipLabel><relationshipName>HbCs</relationshipName><type>MasterDetail</type><reparentableMasterDetail>false</reparentableMasterDetail></CustomField>`)
	write("objects/HbC__c/fields/Amt__c.field-meta.xml", `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Amt__c</fullName><label>Amt</label><precision>10</precision><scale>2</scale><type>Number</type></CustomField>`)
	if withTrigger {
		write("triggers/HbCTrg.trigger", `trigger HbCTrg on HbC__c (after insert) { Integer n = Trigger.new.size(); }`)
		write("triggers/HbCTrg.trigger-meta.xml", gapTriggerMeta(api))
	}
	write("classes/HbTest.cls", `@IsTest private class HbTest { @IsTest static void bulk() {
  HbP__c p = new HbP__c(Name = 'p'); insert p;
  for (Integer i = 0; i < 20; i++) {
    List<HbC__c> cs = new List<HbC__c>();
    for (Integer j = 0; j < 100; j++) { cs.add(new HbC__c(Name = 'c' + j, P__c = p.Id, Amt__c = j)); }
    insert cs;
  }
  System.assertEquals(2000, [SELECT COUNT() FROM HbC__c]);
} }`)
	write("classes/HbTest.cls-meta.xml", gapClassMeta(api))
	index := loadTestIndex(t, root)
	var millis []float64
	for i := 0; i < 9; i++ {
		start := time.Now()
		run := Run(index, Options{SelectedClasses: []string{"HbTest"}, NoDiskCache: true, Parallelism: 1})
		if summary := run.Summary(); summary.Passed != 1 {
			t.Fatalf("summary = %+v", summary)
		}
		millis = append(millis, float64(time.Since(start).Microseconds())/1000)
	}
	sort.Float64s(millis)
	return millis
}

func TestRollupBulkInsertTiming(t *testing.T) {
	if os.Getenv("GLADE_ROLLUP_HOLD_PERF") == "" {
		t.Skip("set GLADE_ROLLUP_HOLD_PERF=1 to time the roll-up bulk insert")
	}
	for _, withTrigger := range []bool{false, true} {
		millis := rollupBulkInsertMillis(t, withTrigger)
		t.Logf("withTrigger=%v median=%.1fms min=%.1fms all=%v", withTrigger, millis[len(millis)/2], millis[0], millis)
	}
}
