public class CalloutMockRecorder implements HttpCalloutMock {
 public List<Map<String,Object>> requests=new List<Map<String,Object>>();
 public Integer statusCode=201;
 public String status='Created';
 public String body='mock-body';
 public HttpResponse respond(HttpRequest request) {
  Map<String,Object> row=new Map<String,Object>{
   'method'=>request.getMethod(),'endpoint'=>request.getEndpoint(),'body'=>request.getBody(),
   'xreq'=>request.getHeader('X-Req')};
  requests.add(row);
  HttpResponse response=new HttpResponse();
  response.setStatusCode(statusCode); response.setStatus(status); response.setBody(body);
  response.setHeader('X-Mock','yes');
  return response;
 }
}
