@IsTest private class Pe06FailThenRepublishTest {
 @IsTest static void observe() {
  Test.startTest();
  EventBus.publish(new SignalImmediate__e(Payload__c='f6-a'));
  Map<String,Object> p=new Map<String,Object>();
  try { Test.getEventBus().fail(); p.put('fail','ok'); } catch(Exception e) { p.put('fail',PeProbe.error(e)); }
  EventBus.publish(new SignalImmediate__e(Payload__c='f6-b'));
  try { Test.getEventBus().deliver(); p.put('deliver','ok'); } catch(Exception e) { p.put('deliver',PeProbe.error(e)); }
  p.put('afterDeliver',PeProbe.names());
  Test.stopTest();
  p.put('afterStop',PeProbe.names());
  System.assert(false,'OBS|'+JSON.serialize(p));
 }
}
