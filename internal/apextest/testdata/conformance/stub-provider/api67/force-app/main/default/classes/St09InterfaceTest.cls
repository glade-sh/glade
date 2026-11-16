@IsTest private class St09InterfaceTest {
 @IsTest static void observe() {
  RecordingProvider p=new RecordingProvider();
  StubIface t=(StubIface)Test.createStub(StubIface.class,p);
  String a=t.run('x');
  String b=t.two('y',2);
  Map<String,Object> o=new Map<String,Object>{'run'=>a,'two'=>b,'providerCalls'=>p.calls.size(),'calls'=>p.calls};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
