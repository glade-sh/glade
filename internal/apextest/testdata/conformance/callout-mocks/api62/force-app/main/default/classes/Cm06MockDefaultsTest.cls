@IsTest private class Cm06MockDefaultsTest {
 @IsTest static void observe() {
  StaticResourceCalloutMock mock=new StaticResourceCalloutMock();
  mock.setStaticResource('MockReply');
  Test.setMock(HttpCalloutMock.class,mock);
  HttpRequest q=new HttpRequest(); q.setEndpoint('https://mock.invalid/defaults'); q.setMethod('GET');
  Integer before=Limits.getCallouts();
  HttpResponse r=new Http().send(q);
  Map<String,Object> o=new Map<String,Object>{'code'=>r.getStatusCode(),'status'=>r.getStatus(),'body'=>r.getBody(),
   'headerNull'=>r.getHeader('X-SR')==null,'delta'=>Limits.getCallouts()-before,'limit'=>Limits.getLimitCallouts()};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
