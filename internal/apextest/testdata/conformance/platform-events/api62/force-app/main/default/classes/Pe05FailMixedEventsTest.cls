@IsTest private class Pe05FailMixedEventsTest {
 @IsTest static void observe() {
  Test.startTest();
  EventBus.publish(new SignalImmediate__e(Payload__c='i5'));
  EventBus.publish(new SignalAfterCommit__e(Payload__c='c5'));
  Map<String,Object> p=new Map<String,Object>();
  try { Test.getEventBus().fail(); p.put('fail','ok'); } catch(Exception e) { p.put('fail',PeProbe.error(e)); }
  p.put('before',PeProbe.names());
  Test.stopTest();
  p.put('after',PeProbe.names());
  System.assert(false,'OBS|'+JSON.serialize(p));
 }
}
