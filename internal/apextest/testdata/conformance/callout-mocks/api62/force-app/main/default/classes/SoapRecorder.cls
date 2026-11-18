public class SoapRecorder implements WebServiceMock {
 public Integer calls=0;
 public Map<String,Object> last=new Map<String,Object>();
 public void doInvoke(Object stub,Object request,Map<String,Object> response,String endpoint,String soapAction,String requestName,String responseNS,String responseName,String responseType) {
  calls++;
  last.put('endpoint',endpoint); last.put('soapAction',soapAction); last.put('requestName',requestName);
  last.put('responseNS',responseNS); last.put('responseName',responseName); last.put('responseType',responseType);
  last.put('stubClass',stub==null?'null':String.valueOf(stub).substringBefore(':'));
  last.put('requestClass',request==null?'null':String.valueOf(request).substringBefore(':'));
  MockSoapService.EchoResponse value=new MockSoapService.EchoResponse(); value.value='soap-response';
  response.put('response_x',value);
 }
}
