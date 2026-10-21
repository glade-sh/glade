package apextest

import (
	"encoding/json"
	"testing"
)

// UNRUN candidate fixture. Only the two retained DEFAULT JSON insert positives
// at source 62/67, transport 67 are asserted. No trigger-placement claim.
func TestFieldWriteabilityRecoveryLastActivityDate(t *testing.T) {
	const className = "ActivityDateRecoveryTest"
	const source = `@IsTest private class ActivityDateRecoveryTest {
 @IsTest static void jsonDateInsert() {
  Account record = (Account)JSON.deserialize('{"Name":"Owned activity date","LastActivityDate":"2007-02-03"}', Account.class);
  Date supplied = Date.newInstance(2007, 2, 3);
  System.assertEquals(supplied, record.LastActivityDate);
  Database.SaveResult result = Database.insert(record, false);
  System.assertEquals(true, result.isSuccess(), JSON.serialize(result.getErrors()));
  Account stored = [SELECT LastActivityDate FROM Account WHERE Id=:record.Id];
  System.assertEquals(supplied, record.LastActivityDate);
  System.assertEquals(null, stored.LastActivityDate);
 }
}`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, map[string]string{className: source}, nil)
			run := Run(index, Options{
				RuntimeRESTAPIVersion: "67.0",
				SelectedClasses:       []string{className},
				NoDiskCache:           true,
				Parallelism:           1,
			})
			if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 {
				data, _ := json.Marshal(run)
				t.Fatalf("retained activity-date positive: %s", data)
			}
		})
	}
}
