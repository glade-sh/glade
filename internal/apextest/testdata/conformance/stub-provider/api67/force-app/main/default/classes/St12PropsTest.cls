@IsTest private class St12PropsTest {
 @IsTest static void observe() {
  RecordingProvider p=new RecordingProvider();
  StubProps t=(StubProps)Test.createStub(StubProps.class,p);
  Map<String,Object> o=new Map<String,Object>();
  try { t.value='written'; o.put('set','ok'); } catch(Exception e) { o.put('set',e.getTypeName()+'|'+e.getMessage()); }
  try { o.put('read',t.value); } catch(Exception e) { o.put('read',e.getTypeName()+'|'+e.getMessage()); }
  o.put('run',t.run()); o.put('providerCalls',p.calls.size()); o.put('calls',p.calls);
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
