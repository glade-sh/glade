@IsTest private class Ao02BatchFinishFutureTest {
 @IsTest static void observe() {
  Test.startTest();
  Id job=Database.executeBatch(new BatchFuture('a2'),1);
  Map<String,Object> o=new Map<String,Object>{'submitted'=>job!=null};
  try { Test.stopTest(); o.put('stop','ok'); } catch(Exception e) { o.put('stop',e.getTypeName()+'|'+e.getMessage()); }
  o.put('rows',AsyncProbe.read());
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
