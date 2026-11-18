@IsTest private class St01StaticTest {
 @IsTest static void observe() {
  StubTarget.staticCalls=0;
  RecordingProvider p=new RecordingProvider();
  StubTarget t=(StubTarget)Test.createStub(StubTarget.class,p);
  String value=StubTarget.stat('x');
  Map<String,Object> o=new Map<String,Object>{'value'=>value,'staticCalls'=>StubTarget.staticCalls,'providerCalls'=>p.calls.size(),'calls'=>p.calls};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
