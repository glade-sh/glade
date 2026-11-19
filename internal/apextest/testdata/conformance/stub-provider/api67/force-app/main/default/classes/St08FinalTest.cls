@IsTest private class St08FinalTest {
 @IsTest static void observe() {
  RecordingProvider p=new RecordingProvider();
  Map<String,Object> o=new Map<String,Object>();
  try { Object t=Test.createStub(StubFinal.class,p); o.put('created',true); o.put('value',((StubFinal)t).run()); }
  catch(Exception e) { o.put('created',false); o.put('error',e.getTypeName()+'|'+e.getMessage()); }
  o.put('providerCalls',p.calls.size());
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
