@IsTest private class Cm03MultiStaticResourceMockTest {
 @IsTest static void observe() {
  MultiStaticResourceCalloutMock mock=new MultiStaticResourceCalloutMock();
  mock.setStaticResource('https://mock.invalid/a','MultiA');
  mock.setStaticResource('https://mock.invalid/b','MultiB');
  Test.setMock(HttpCalloutMock.class,mock);
  Map<String,Object> o=new Map<String,Object>();
  for (String path: new List<String>{'a','b'}) {
   HttpRequest q=new HttpRequest(); q.setEndpoint('https://mock.invalid/'+path); q.setMethod('GET');
   Integer before=Limits.getCallouts();
   HttpResponse r=new Http().send(q);
   o.put(path,r.getStatusCode()+'|'+r.getBody()+'|'+(Limits.getCallouts()-before));
  }
  HttpRequest missing=new HttpRequest(); missing.setEndpoint('https://mock.invalid/missing'); missing.setMethod('GET');
  try { HttpResponse r=new Http().send(missing); o.put('missing',r==null?'null':r.getStatusCode()+'|'+r.getBody()); }
  catch(Exception e) { o.put('missing',e.getTypeName()+'|'+e.getMessage()); }
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
