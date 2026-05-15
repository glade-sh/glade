@isTest
private class OrgLikeRunnerTest {
  @TestSetup static void seed() {
    insert new Widget__c(Name = 'Seed');
    RunnerState.value = 99;
  }

  @isTest static void setupDataAndStaticsAreIsolated() {
    System.assertEquals(1, RunnerState.value);
    RunnerState.value = 5;

    Widget__c seed = [SELECT Name FROM Widget__c WHERE Name = 'Seed' LIMIT 1];
    seed.Name = 'Changed';
    update seed;
    insert new Widget__c(Name = 'Extra');
    List<Widget__c> allRows = [SELECT Id FROM Widget__c];
    System.assertEquals(2, allRows.size());
  }

  @isTest static void saveOrderAsyncAndRollbackCompose() {
    System.assertEquals(1, RunnerState.value);
    List<Widget__c> seeds = [SELECT Id FROM Widget__c WHERE Name = 'Seed'];
    List<Widget__c> changed = [SELECT Id FROM Widget__c WHERE Name = 'Changed'];
    List<Widget__c> extras = [SELECT Id FROM Widget__c WHERE Name = 'Extra'];
    System.assertEquals(1, seeds.size());
    System.assertEquals(0, changed.size());
    System.assertEquals(0, extras.size());

    Test.startTest();
    RunnerFuture.mark(7);
    String batchId = Database.executeBatch(new RunnerBatch(), 2);
    String scheduleId = System.schedule('nightly', '0 0 0 * * ?', new RunnerSchedule());
    System.enqueueJob(new RunnerQueue());
    Database.SaveResult eventResult = EventBus.publish(new Widget_Event__e(Name__c = 'Local Event'));
    System.assert(eventResult.isSuccess());
    Test.getEventBus().deliver();
    System.assertEquals(1, RunnerState.futureRan);
    System.assertNotEquals('', batchId);
    System.assertNotEquals('', scheduleId);
    insert new Widget__c(Name = 'Acme');
    System.assertEquals(0, RunnerState.asyncRan);
    List<Widget__c> beforeFuture = [SELECT Id FROM Widget__c WHERE Name = 'Future-7'];
    List<Widget__c> beforeBatch = [SELECT Id FROM Widget__c WHERE Name = 'Batch-1'];
    List<Widget__c> beforeScheduled = [SELECT Id FROM Widget__c WHERE Name = 'Scheduled'];
    List<Widget__c> beforeQueueable = [SELECT Id FROM Widget__c WHERE Name = 'Queueable'];
    System.assertEquals(0, beforeFuture.size());
    System.assertEquals(0, beforeBatch.size());
    System.assertEquals(0, beforeScheduled.size());
    System.assertEquals(0, beforeQueueable.size());
    Test.stopTest();
    List<Widget__c> futureRows = [SELECT Id FROM Widget__c WHERE Name = 'Future-7'];
    List<Widget__c> batchRows = [SELECT Id FROM Widget__c WHERE Name = 'Batch-1' OR Name = 'Batch-2' OR Name = 'Batch-3'];
    List<Widget__c> batchFinishRows = [SELECT Id FROM Widget__c WHERE Name = 'Batch-Finish'];
    List<Widget__c> scheduledRows = [SELECT Id FROM Widget__c WHERE Name = 'Scheduled'];
    List<Widget__c> queueableRows = [SELECT Id FROM Widget__c WHERE Name = 'Queueable'];
    System.assertEquals(1, futureRows.size());
    System.assertEquals(3, batchRows.size());
    System.assertEquals(1, batchFinishRows.size());
    System.assertEquals(1, scheduledRows.size());
    System.assertEquals(1, queueableRows.size());

    List<CronTrigger> crons = [SELECT Id, State FROM CronTrigger];
    System.assertEquals(1, crons.size());
    System.assertEquals('Complete', crons.get(0).State);

    Widget__c row = [SELECT Before__c, Status__c, Flow__c FROM Widget__c WHERE Name = 'Acme' LIMIT 1];
    System.assertEquals('before', row.Before__c);
    System.assertEquals('workflow', row.Status__c);
    System.assertEquals('flow', row.Flow__c);

    System.Savepoint sp = Database.setSavepoint();
    insert new Widget__c(Name = 'Rollback');
    List<Widget__c> beforeRollback = [SELECT Id FROM Widget__c WHERE Name = 'Rollback'];
    System.assertEquals(1, beforeRollback.size());
    Database.rollback(sp);
    List<Widget__c> afterRollback = [SELECT Id FROM Widget__c WHERE Name = 'Rollback'];
    System.assertEquals(0, afterRollback.size());
  }
}
