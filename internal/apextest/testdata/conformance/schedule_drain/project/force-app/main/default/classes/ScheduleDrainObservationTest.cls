@IsTest private class ScheduleDrainObservationTest {
 @IsTest static void directTopLevel() {
  Test.startTest();
  ScheduleDrainProbe.scheduleWork(false);
  Test.stopTest();
  Map<String,Object> observed = new Map<String,Object>{
   'scheduled' => [SELECT count() FROM Account WHERE Name='Schedule drain ran'],
   'batch' => [SELECT count() FROM Account WHERE Name='Schedule batch finished'],
   'queue' => [SELECT count() FROM Account WHERE Name='Schedule queue ran'],
   'relay' => [SELECT count() FROM Account WHERE Name='Schedule relay ran'],
   'cron' => [SELECT count() FROM CronTrigger]
  };
  System.assert(false,'OBSERVED|directTopLevel|' + JSON.serialize(observed));
 }
 @IsTest static void directInnerClass() {
  Test.startTest();
  ScheduleDrainProbe.scheduleWork(true);
  Test.stopTest();
  Map<String,Object> observed = new Map<String,Object>{
   'scheduled' => [SELECT count() FROM Account WHERE Name='Schedule drain ran'],
   'batch' => [SELECT count() FROM Account WHERE Name='Schedule batch finished'],
   'queue' => [SELECT count() FROM Account WHERE Name='Schedule queue ran'],
   'relay' => [SELECT count() FROM Account WHERE Name='Schedule relay ran'],
   'cron' => [SELECT count() FROM CronTrigger]
  };
  System.assert(false,'OBSERVED|directInnerClass|' + JSON.serialize(observed));
 }
 @IsTest static void batchFinishTopLevel() {
  Test.startTest();
  Database.executeBatch(new ScheduleDrainBatch(false));
  Test.stopTest();
  Map<String,Object> observed = new Map<String,Object>{
   'scheduled' => [SELECT count() FROM Account WHERE Name='Schedule drain ran'],
   'batch' => [SELECT count() FROM Account WHERE Name='Schedule batch finished'],
   'queue' => [SELECT count() FROM Account WHERE Name='Schedule queue ran'],
   'relay' => [SELECT count() FROM Account WHERE Name='Schedule relay ran'],
   'cron' => [SELECT count() FROM CronTrigger]
  };
  System.assert(false,'OBSERVED|batchFinishTopLevel|' + JSON.serialize(observed));
 }
 @IsTest static void batchFinishInnerClass() {
  Test.startTest();
  Database.executeBatch(new ScheduleDrainBatch(true));
  Test.stopTest();
  Map<String,Object> observed = new Map<String,Object>{
   'scheduled' => [SELECT count() FROM Account WHERE Name='Schedule drain ran'],
   'batch' => [SELECT count() FROM Account WHERE Name='Schedule batch finished'],
   'queue' => [SELECT count() FROM Account WHERE Name='Schedule queue ran'],
   'relay' => [SELECT count() FROM Account WHERE Name='Schedule relay ran'],
   'cron' => [SELECT count() FROM CronTrigger]
  };
  System.assert(false,'OBSERVED|batchFinishInnerClass|' + JSON.serialize(observed));
 }
 @IsTest static void queueableTopLevel() {
  Test.startTest();
  System.enqueueJob(new ScheduleDrainProbe.QueueWork(false));
  Test.stopTest();
  Map<String,Object> observed = new Map<String,Object>{
   'scheduled' => [SELECT count() FROM Account WHERE Name='Schedule drain ran'],
   'batch' => [SELECT count() FROM Account WHERE Name='Schedule batch finished'],
   'queue' => [SELECT count() FROM Account WHERE Name='Schedule queue ran'],
   'relay' => [SELECT count() FROM Account WHERE Name='Schedule relay ran'],
   'cron' => [SELECT count() FROM CronTrigger]
  };
  System.assert(false,'OBSERVED|queueableTopLevel|' + JSON.serialize(observed));
 }
 @IsTest static void queueableInnerClass() {
  Test.startTest();
  System.enqueueJob(new ScheduleDrainProbe.QueueWork(true));
  Test.stopTest();
  Map<String,Object> observed = new Map<String,Object>{
   'scheduled' => [SELECT count() FROM Account WHERE Name='Schedule drain ran'],
   'batch' => [SELECT count() FROM Account WHERE Name='Schedule batch finished'],
   'queue' => [SELECT count() FROM Account WHERE Name='Schedule queue ran'],
   'relay' => [SELECT count() FROM Account WHERE Name='Schedule relay ran'],
   'cron' => [SELECT count() FROM CronTrigger]
  };
  System.assert(false,'OBSERVED|queueableInnerClass|' + JSON.serialize(observed));
 }
 @IsTest static void scheduledCallbackTopLevel() {
  Test.startTest();
  System.schedule('Drain parent','0 0 0 1 1 ? 2099',new ScheduleDrainRelay(false));
  Test.stopTest();
  Map<String,Object> observed = new Map<String,Object>{
   'scheduled' => [SELECT count() FROM Account WHERE Name='Schedule drain ran'],
   'batch' => [SELECT count() FROM Account WHERE Name='Schedule batch finished'],
   'queue' => [SELECT count() FROM Account WHERE Name='Schedule queue ran'],
   'relay' => [SELECT count() FROM Account WHERE Name='Schedule relay ran'],
   'cron' => [SELECT count() FROM CronTrigger]
  };
  System.assert(false,'OBSERVED|scheduledCallbackTopLevel|' + JSON.serialize(observed));
 }
 @IsTest static void scheduledCallbackInnerClass() {
  Test.startTest();
  System.schedule('Drain parent','0 0 0 1 1 ? 2099',new ScheduleDrainRelay(true));
  Test.stopTest();
  Map<String,Object> observed = new Map<String,Object>{
   'scheduled' => [SELECT count() FROM Account WHERE Name='Schedule drain ran'],
   'batch' => [SELECT count() FROM Account WHERE Name='Schedule batch finished'],
   'queue' => [SELECT count() FROM Account WHERE Name='Schedule queue ran'],
   'relay' => [SELECT count() FROM Account WHERE Name='Schedule relay ran'],
   'cron' => [SELECT count() FROM CronTrigger]
  };
  System.assert(false,'OBSERVED|scheduledCallbackInnerClass|' + JSON.serialize(observed));
 }
}
