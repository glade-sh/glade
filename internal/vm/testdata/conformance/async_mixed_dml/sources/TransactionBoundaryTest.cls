@IsTest public class TransactionBoundaryTest {
@IsTest static void queueable() {
        TransactionDml.dataRow('Parent');
        Test.startTest();
        System.enqueueJob(new TransactionQueue(true,'QueueSetup',false));
System.enqueueJob(new TransactionQueue(false,'QueueData',false));
        Test.stopTest();
        TransactionDml.rejectOpposite(false,'ParentAfter');
        System.assertEquals(2,[SELECT COUNT() FROM Account WHERE Name LIKE 'Boundary %']);
        System.assertEquals(1,[SELECT COUNT() FROM PermissionSet WHERE Name LIKE 'Boundary%']);
        System.assert(false,'OBS|"success"');
    }
@IsTest static void future() {
        TransactionDml.dataRow('Parent');
        Test.startTest();
        TransactionDml.later(true,'FutureSetup');
TransactionDml.later(false,'FutureData');
        Test.stopTest();
        TransactionDml.rejectOpposite(false,'ParentAfter');
        System.assertEquals(2,[SELECT COUNT() FROM Account WHERE Name LIKE 'Boundary %']);
        System.assertEquals(1,[SELECT COUNT() FROM PermissionSet WHERE Name LIKE 'Boundary%']);
        System.assert(false,'OBS|"success"');
    }
@IsTest static void scheduled() {
        TransactionDml.dataRow('Parent');
        Test.startTest();
        System.schedule('Setup','0 0 0 1 1 ? 2099',new TransactionSchedule(true,'ScheduleSetup'));
System.schedule('Data','0 0 0 1 1 ? 2099',new TransactionSchedule(false,'ScheduleData'));
        Test.stopTest();
        TransactionDml.rejectOpposite(false,'ParentAfter');
        System.assertEquals(2,[SELECT COUNT() FROM Account WHERE Name LIKE 'Boundary %']);
        System.assertEquals(1,[SELECT COUNT() FROM PermissionSet WHERE Name LIKE 'Boundary%']);
        System.assert(false,'OBS|"success"');
    }
@IsTest static void batch() {
        TransactionDml.dataRow('Parent');
        Test.startTest();
        Database.executeBatch(new TransactionBatch(),1);
        Test.stopTest();
        TransactionDml.rejectOpposite(false,'ParentAfter');
        System.assertEquals(2,[SELECT COUNT() FROM Account WHERE Name LIKE 'Boundary %']);
        System.assertEquals(2,[SELECT COUNT() FROM PermissionSet WHERE Name LIKE 'Boundary%']);
        System.assert(false,'OBS|"success"');
    }
@IsTest static void scheduled_batch() {
        TransactionDml.dataRow('Parent');
        System.scheduleBatch(new TransactionBatch(),'Batch',1,1);
        Test.startTest();
        Test.stopTest();
        TransactionDml.rejectOpposite(false,'ParentAfter');
        System.assertEquals(1,[SELECT COUNT() FROM Account WHERE Name LIKE 'Boundary %']);
        System.assertEquals(0,[SELECT COUNT() FROM PermissionSet WHERE Name LIKE 'Boundary%']);
        System.assert(false,'OBS|"success"');
    }
@IsTest static void finalizer() {
        TransactionDml.dataRow('Parent');
        Test.startTest();
        System.enqueueJob(new TransactionQueue(true,'QueueSetup',true));
        Boolean rejected=false;
        try { Test.stopTest(); } catch(DmlException e) {
            rejected=String.valueOf(e.getDmlType(0))=='MIXED_DML_OPERATION';
        }
        System.assert(rejected,'test finalizer retains parent Queueable mixed DML');
        System.assertEquals(1,[SELECT COUNT() FROM Account WHERE Name LIKE 'Boundary %']);
        System.assertEquals(1,[SELECT COUNT() FROM PermissionSet WHERE Name LIKE 'Boundary%']);
        TransactionDml.rejectOpposite(false,'ParentAfter');
        System.assert(false,'OBS|"mixed_dml"');
    }
}
