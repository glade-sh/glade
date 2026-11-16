@IsTest private class Ao01QueueBatchTest {
 @IsTest static void observe() {
  Test.startTest();
  Id job=System.enqueueJob(new QueueBatch('a1'));
  Map<String,Object> o=new Map<String,Object>{'submitted'=>job!=null};
  try { Test.stopTest(); o.put('stop','ok'); } catch(Exception e) { o.put('stop',e.getTypeName()+'|'+e.getMessage()); }
  o.put('rows',AsyncProbe.read());
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
