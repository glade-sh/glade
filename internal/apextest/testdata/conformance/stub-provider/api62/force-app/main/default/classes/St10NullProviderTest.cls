@IsTest private class St10NullProviderTest {
 @IsTest static void observe() {
  Map<String,Object> o=new Map<String,Object>();
  try { Object t=Test.createStub(StubTarget.class,null); o.put('created',true); }
  catch(Exception e) { o.put('created',false); o.put('error',e.getTypeName()+'|'+e.getMessage()); }
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
