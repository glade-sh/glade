@IsTest private class SubscriberNoRowsProbeTest {
 @IsTest static void observe() { System.assert(false,'OBS|'+SubscriberNoRowsProbe.sample()); }
 @IsTest static void observeDelivery() {
  EventBusSubscriber beforeRow=[SELECT Position FROM EventBusSubscriber WHERE Topic='SyntheticEvent__e' AND Type='ApexTrigger'];
  Integer beforePosition=beforeRow.Position;
  Test.startTest(); Database.SaveResult result=EventBus.publish(new SyntheticEvent__e()); Test.getEventBus().deliver();
  EventBusSubscriber midRow=[SELECT Position FROM EventBusSubscriber WHERE Topic='SyntheticEvent__e' AND Type='ApexTrigger'];
  List<Database.SaveResult> batch=EventBus.publish(new List<SyntheticEvent__e>{new SyntheticEvent__e(),new SyntheticEvent__e()}); Test.stopTest();
  EventBusSubscriber afterRow=[SELECT Position FROM EventBusSubscriber WHERE Topic='SyntheticEvent__e' AND Type='ApexTrigger'];
  EventBusSubscriber otherRow=[SELECT Position FROM EventBusSubscriber WHERE Topic='SecondEvent__e' AND Type='ApexTrigger'];
  System.assert(false,'DELIVERY|'+JSON.serialize(new Map<String,Object>{'before'=>beforePosition,'mid'=>midRow.Position,'after'=>afterRow.Position,'other'=>otherRow.Position,'published'=>result.isSuccess(),'batchSize'=>batch.size()}));
 }
}
