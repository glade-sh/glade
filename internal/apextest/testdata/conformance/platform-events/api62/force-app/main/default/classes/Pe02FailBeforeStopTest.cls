@IsTest private class Pe02FailBeforeStopTest {
 @IsTest static void observe() {
  Test.startTest();
  Database.SaveResult r=EventBus.publish(new SignalImmediate__e(Payload__c='f2'));
  Map<String,Object> p=new Map<String,Object>{'pub'=>r.isSuccess()};
  try { Test.getEventBus().fail(); p.put('fail','ok'); } catch(Exception e) { p.put('fail',PeProbe.error(e)); }
  p.put('before',PeProbe.names());
  Test.stopTest();
  p.put('after',PeProbe.names());
  System.assert(false,'OBS|'+JSON.serialize(p));
 }
}
