@IsTest public class CallbackDispatchProbe {
    @IsTest static void asynchronous() {
        Test.startTest();
        System.enqueueJob(new WorkBatch());
        Test.stopTest();
        System.assertEquals(1, [SELECT count() FROM Account WHERE Name='callback completed']);
    }
    @IsTest static void direct() {
        WorkBatch work = new WorkBatch();
        CompletionSink callback = work.createCallback();
        work.setCallback(callback);
        work.finish();
        System.assertEquals(1, callback.count);
    }
    @IsTest static void reentrant() {
        WorkBatch work = new WorkBatch();
        CompletionSink callback = work.createCallback();
        work.setCallback(callback);
        callback.execute(null);
        System.assertEquals(1, callback.count);
    }
}
