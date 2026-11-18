@IsTest private class Pe07FailCdcTest {
 @IsTest static void observe() {
  Test.enableChangeDataCapture();
  Test.startTest();
  insert new Account(Name='cdc-fail');
  Map<String,Object> p=new Map<String,Object>();
  p.put('before',[SELECT count() FROM EventLog__c]);
  try { Test.getEventBus().fail(); p.put('fail','ok'); } catch(Exception e) { p.put('fail',PeProbe.error(e)); }
  try { Test.getEventBus().deliver(); p.put('deliver','ok'); } catch(Exception e) { p.put('deliver',PeProbe.error(e)); }
  p.put('afterDeliver',[SELECT count() FROM EventLog__c]);
  Test.stopTest();
  p.put('afterStop',[SELECT count() FROM EventLog__c]);
  System.assert(false,'OBS|'+JSON.serialize(p));
 }
}
