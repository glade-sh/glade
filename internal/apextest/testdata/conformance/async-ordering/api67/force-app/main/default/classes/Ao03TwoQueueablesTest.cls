@IsTest private class Ao03TwoQueueablesTest {
 @IsTest static void observe() {
  Test.startTest();
  Id first=System.enqueueJob(new LogQueue('first'));
  Map<String,Object> o=new Map<String,Object>{'first'=>(first!=null)};
  try { Id second=System.enqueueJob(new LogQueue('second')); o.put('second',second!=null); }
  catch(Exception e) { o.put('second',e.getTypeName()+'|'+e.getMessage()); }
  try { Test.stopTest(); o.put('stop','ok'); } catch(Exception e) { o.put('stop',e.getTypeName()+'|'+e.getMessage()); }
  o.put('rows',AsyncProbe.read());
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
