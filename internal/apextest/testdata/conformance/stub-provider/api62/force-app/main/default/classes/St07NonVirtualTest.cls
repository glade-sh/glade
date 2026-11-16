@IsTest private class St07NonVirtualTest {
 @IsTest static void observe() {
  RecordingProvider p=new RecordingProvider();
  StubTarget t=(StubTarget)Test.createStub(StubTarget.class,p);
  String value=t.nonVirtual('x');
  Map<String,Object> o=new Map<String,Object>{'value'=>value,'nonVirtualCalls'=>t.nonVirtualCalls,'providerCalls'=>p.calls.size(),'calls'=>p.calls};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
