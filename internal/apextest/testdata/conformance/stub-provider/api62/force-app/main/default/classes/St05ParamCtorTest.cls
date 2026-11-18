@IsTest private class St05ParamCtorTest {
 @IsTest static void observe() {
  RecordingProvider p=new RecordingProvider();
  Map<String,Object> o=new Map<String,Object>();
  try {
   StubOnlyParamCtor t=(StubOnlyParamCtor)Test.createStub(StubOnlyParamCtor.class,p);
   o.put('created',true); o.put('value',t.run('x')); o.put('ctorField',t.ctors); o.put('ctorCount',StubOnlyParamCtor.ctorCount);
  } catch(Exception e) { o.put('created',false); o.put('error',e.getTypeName()+'|'+e.getMessage()); }
  o.put('providerCalls',p.calls.size()); o.put('calls',p.calls);
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
