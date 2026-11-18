@IsTest private class St06AbstractTest {
 @IsTest static void observe() {
  RecordingProvider p=new RecordingProvider();
  StubAbstract t=(StubAbstract)Test.createStub(StubAbstract.class,p);
  String a=t.run('x');
  String b=t.tag();
  Map<String,Object> o=new Map<String,Object>{'run'=>a,'tag'=>b,'providerCalls'=>p.calls.size(),'calls'=>p.calls};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
