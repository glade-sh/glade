@IsTest private class St04ConstructorTest {
 @IsTest static void observe() {
  RecordingProvider p=new RecordingProvider();
  StubTarget t=(StubTarget)Test.createStub(StubTarget.class,p);
  Map<String,Object> o=new Map<String,Object>{'ctorField'=>t.ctors,'ctorCount'=>StubTarget.ctorCount,'isTarget'=>((Object)t) instanceof StubTarget,'providerCalls'=>p.calls.size(),'calls'=>p.calls};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
