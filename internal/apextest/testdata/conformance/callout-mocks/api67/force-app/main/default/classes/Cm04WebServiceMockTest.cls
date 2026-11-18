@IsTest private class Cm04WebServiceMockTest {
 @IsTest static void observe() {
  SoapRecorder mock=new SoapRecorder();
  Test.setMock(WebServiceMock.class,mock);
  Integer before=Limits.getCallouts();
  Map<String,Object> o=new Map<String,Object>();
  try { o.put('value',new MockSoapService().call('request')); }
  catch(Exception e) { o.put('error',e.getTypeName()+'|'+e.getMessage()); }
  o.put('delta',Limits.getCallouts()-before); o.put('calls',mock.calls); o.put('last',mock.last);
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
