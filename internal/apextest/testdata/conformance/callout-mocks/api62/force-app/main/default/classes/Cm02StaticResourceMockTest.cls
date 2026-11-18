@IsTest private class Cm02StaticResourceMockTest {
 @IsTest static void observe() {
  StaticResourceCalloutMock mock=new StaticResourceCalloutMock();
  mock.setStaticResource('MockReply'); mock.setStatusCode(202); mock.setStatus('Accepted'); mock.setHeader('X-SR','on');
  Test.setMock(HttpCalloutMock.class,mock);
  HttpRequest q=new HttpRequest(); q.setEndpoint('https://mock.invalid/sr'); q.setMethod('GET');
  Integer before=Limits.getCallouts();
  HttpResponse r=new Http().send(q);
  Integer after=Limits.getCallouts();
  Map<String,Object> o=new Map<String,Object>{'code'=>r.getStatusCode(),'status'=>r.getStatus(),'body'=>r.getBody(),
   'header'=>r.getHeader('X-SR'),'delta'=>after-before};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
