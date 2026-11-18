@IsTest private class Pe08FailMixedCdcTest {
 @IsTest static void observe() {
  Test.enableChangeDataCapture();
  Test.startTest();
  EventBus.publish(new SignalImmediate__e(Payload__c='mix'));
  insert new Account(Name='cdc-mix');
  Map<String,Object> p=new Map<String,Object>();
  try { Test.getEventBus().fail(); p.put('fail','ok'); } catch(Exception e) { p.put('fail',PeProbe.error(e)); }
  p.put('before',[SELECT count() FROM EventLog__c]+PeProbe.names().size());
  Test.stopTest();
  p.put('cdc',[SELECT count() FROM EventLog__c]); p.put('pe',PeProbe.names());
  System.assert(false,'OBS|'+JSON.serialize(p));
 }
}
