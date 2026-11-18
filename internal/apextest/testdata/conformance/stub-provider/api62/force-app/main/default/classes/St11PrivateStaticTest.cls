@IsTest private class St11PrivateStaticTest {
 @IsTest static void observe() {
  StubTarget.privateStaticCalls=0;
  RecordingProvider p=new RecordingProvider();
  StubTarget t=(StubTarget)Test.createStub(StubTarget.class,p);
  String value=StubTarget.callHiddenStatic('x');
  Map<String,Object> o=new Map<String,Object>{'value'=>value,'privateStaticCalls'=>StubTarget.privateStaticCalls,'providerCalls'=>p.calls.size(),'calls'=>p.calls};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
