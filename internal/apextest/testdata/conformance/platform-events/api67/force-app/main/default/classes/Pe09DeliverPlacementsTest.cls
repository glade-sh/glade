@IsTest private class Pe09DeliverPlacementsTest {
 @IsTest static void observe() {
  EventBus.publish(new SignalImmediate__e(Payload__c='outside'));
  Test.getEventBus().deliver();
  Map<String,Object> p=new Map<String,Object>{'beforeStart'=>PeProbe.names()};
  Test.startTest();
  EventBus.publish(new SignalImmediate__e(Payload__c='inside'));
  Test.stopTest();
  p.put('afterStop',PeProbe.names());
  Test.getEventBus().deliver();
  p.put('afterExtra',PeProbe.names());
  System.assert(false,'OBS|'+JSON.serialize(p));
 }
}
