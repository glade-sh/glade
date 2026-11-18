@IsTest private class Cm05NoMockHttpTest {
 @IsTest static void observe() {
  HttpRequest q=new HttpRequest(); q.setEndpoint('https://mock.invalid/none'); q.setMethod('GET');
  Map<String,Object> o=new Map<String,Object>();
  Integer before=Limits.getCallouts();
  try { HttpResponse r=new Http().send(q); o.put('result',r.getStatusCode()); }
  catch(Exception e) { o.put('error',e.getTypeName()+'|'+e.getMessage()); }
  o.put('delta',Limits.getCallouts()-before);
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
