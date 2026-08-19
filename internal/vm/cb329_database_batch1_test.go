package vm

import "testing"

func TestCB329DatabaseBatch1CursorDeleteFilterEnum(t *testing.T) {
	program, err := CompileAnonymous(`
System.assertEquals(Database.Cursor.DeleteFilter.DELETED_ROWS_ONLY, Database.Cursor.DeleteFilter.valueOf('DELETED_ROWS_ONLY'));
System.assertEquals(Database.Cursor.DeleteFilter.DELETED_ROWS_ONLY.hashCode(), Database.Cursor.DeleteFilter.valueOf('DELETED_ROWS_ONLY').hashCode());
System.assertEquals(Database.Cursor.DeleteFilter.NO_DELETED_ROWS, Database.Cursor.DeleteFilter.values()[1]);
System.assertEquals(0, Database.Cursor.DeleteFilter.DELETED_ROWS_ONLY.ordinal());
System.assertEquals('DELETED_ROWS_ONLY', Database.Cursor.DeleteFilter.DELETED_ROWS_ONLY.name());
System.assertEquals(4, Database.Cursor.DeleteFilter.values().size());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestCB329DatabaseBatch1Constructors(t *testing.T) {
	program, err := CompileAnonymous(`
Database.Cursor cursor = new Database.Cursor();
System.assertNotEquals(null, cursor);

Database.CursorFetchResult fetchResult = new Database.CursorFetchResult();
System.assertEquals(0, fetchResult.getNumDeletedRecords());

Database.DeletedRecord deletedRecord = new Database.DeletedRecord();
System.assertEquals(null, deletedRecord.getId());

Database.EmptyRecycleBinResult emptyResult = new Database.EmptyRecycleBinResult();
System.assertEquals(false, emptyResult.isSuccess());
System.assertEquals(0, emptyResult.getErrors().size());

Database.DeleteResult deleteResult = new Database.DeleteResult();
System.assertEquals(false, deleteResult.isSuccess());

Database.GetDeletedResult getDeletedResult = new Database.GetDeletedResult();
System.assertEquals(0, getDeletedResult.getDeletedRecords().size());

Database.GetUpdatedResult getUpdatedResult = new Database.GetUpdatedResult();
System.assertEquals(0, getUpdatedResult.getIds().size());

Database.MergeResult mergeResult = new Database.MergeResult();
System.assertEquals(false, mergeResult.isSuccess());
System.assertEquals(0, mergeResult.getMergedRecordIds().size());

Database.BatchableContextImpl batchCtx = new Database.BatchableContextImpl();
System.assertEquals('', batchCtx.getJobId());
System.assertEquals('', batchCtx.getChildJobId());

Database.DuplicateError dupError = new Database.DuplicateError();
System.assertEquals(null, dupError.getMessage());
System.assertEquals(0, dupError.getFields().size());

Database.AssignmentRuleHeader assignHeader = new Database.AssignmentRuleHeader();
System.assertEquals(null, assignHeader.AssignmentRuleId);

Database.DuplicateRuleHeader dupHeader = new Database.DuplicateRuleHeader();
System.assertEquals(null, dupHeader.AllowSave);

Database.EmailHeader emailHeader = new Database.EmailHeader();
System.assertEquals(null, emailHeader.TriggerAutoResponseEmail);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
