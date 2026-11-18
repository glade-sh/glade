@IsTest private class St03PrivateTest {
 @IsTest static void observe() {
  RecordingProvider p=new RecordingProvider();
  StubTarget t=(StubTarget)Test.createStub(StubTarget.class,p);
  String value=t.callHidden('x');
  Map<String,Object> o=new Map<String,Object>{'value'=>value,'privateCalls'=>t.privateCalls,'providerCalls'=>p.calls.size(),'calls'=>p.calls};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
