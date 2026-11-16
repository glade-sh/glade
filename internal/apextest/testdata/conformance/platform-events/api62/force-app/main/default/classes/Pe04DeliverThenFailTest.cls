@IsTest private class Pe04DeliverThenFailTest {
 @IsTest static void observe() {
  Test.startTest();
  EventBus.publish(new SignalImmediate__e(Payload__c='f4'));
  Map<String,Object> p=new Map<String,Object>();
  try { Test.getEventBus().deliver(); p.put('deliver','ok'); } catch(Exception e) { p.put('deliver',PeProbe.error(e)); }
  p.put('afterDeliver',PeProbe.names());
  try { Test.getEventBus().fail(); p.put('fail','ok'); } catch(Exception e) { p.put('fail',PeProbe.error(e)); }
  p.put('afterFail',PeProbe.names());
  Test.stopTest();
  p.put('afterStop',PeProbe.names());
  System.assert(false,'OBS|'+JSON.serialize(p));
 }
}
