@IsTest private class Cm07SetMockReplaceTest {
 @IsTest static void observe() {
  CalloutMockRecorder first=new CalloutMockRecorder(); first.body='first-body';
  CalloutMockRecorder second=new CalloutMockRecorder(); second.body='second-body';
  Test.setMock(HttpCalloutMock.class,first);
  Test.setMock(HttpCalloutMock.class,second);
  HttpRequest q=new HttpRequest(); q.setEndpoint('https://mock.invalid/replace'); q.setMethod('GET');
  HttpResponse r=new Http().send(q);
  Map<String,Object> o=new Map<String,Object>{'body'=>r.getBody(),'firstCalls'=>first.requests.size(),'secondCalls'=>second.requests.size()};
  System.assert(false,'OBS|'+JSON.serialize(o));
 }
}
