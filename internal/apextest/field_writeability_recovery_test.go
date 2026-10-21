package apextest

import (
	"encoding/json"
	"testing"
)

func TestFieldWriteabilityRecoveryCreationTimestamp(t *testing.T) {
	// Native named-test observations at source APIs 62/67, transport 67,
	// accept JSON-populated creation timestamps without storing the supplied
	// timestamp. Describe remains non-createable and non-updateable.
	const className = "CreationTimestampRecoveryTest"
	const source = `@IsTest private class CreationTimestampRecoveryTest {
 @IsTest static void ordinaryInsert() {
  Account record = new Account(Name='Ordinary timestamp control');
  insert record;
  Account stored = [SELECT CreatedDate FROM Account WHERE Id=:record.Id];
  System.assertNotEquals(null, stored.CreatedDate);
 }
 @IsTest static void jsonTimestamp() {
  System.assertEquals(false, Account.CreatedDate.getDescribe().isCreateable());
  System.assertEquals(false, Account.CreatedDate.getDescribe().isUpdateable());
  Account record = (Account)JSON.deserialize('{"Name":"JSON timestamp control","CreatedDate":"2001-02-03T04:05:06.000Z"}', Account.class);
  Datetime supplied = record.CreatedDate;
  Database.SaveResult result = Database.insert(record, false);
  System.assertEquals(true, result.isSuccess(), JSON.serialize(result.getErrors()));
  Account stored = [SELECT CreatedDate FROM Account WHERE Id=:record.Id];
  System.assertEquals(supplied, record.CreatedDate);
  System.assertNotEquals(supplied, stored.CreatedDate);
 }
 @IsTest static void jsonNullTimestamp() {
  Account record = (Account)JSON.deserialize('{"Name":"JSON null timestamp control","CreatedDate":null}', Account.class);
  Database.SaveResult result = Database.insert(record, false);
  System.assertEquals(true, result.isSuccess(), JSON.serialize(result.getErrors()));
  Account stored = [SELECT CreatedDate FROM Account WHERE Id=:record.Id];
  System.assertNotEquals(null, stored.CreatedDate);
 }
 @IsTest static void postInsertTestSetter() {
  Account record = new Account(Name='Test setter control');
  insert record;
  Datetime supplied = Datetime.newInstanceGmt(2001, 2, 3, 4, 5, 6);
  Test.setCreatedDate(record.Id, supplied);
  Account stored = [SELECT CreatedDate FROM Account WHERE Id=:record.Id];
  System.assertEquals(supplied, stored.CreatedDate);
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
			if summary := run.Summary(); summary.Total != 4 || summary.Passed != 4 {
				data, _ := json.Marshal(run)
				t.Fatalf("creation timestamp named controls: %s", data)
			}
		})
	}
}
