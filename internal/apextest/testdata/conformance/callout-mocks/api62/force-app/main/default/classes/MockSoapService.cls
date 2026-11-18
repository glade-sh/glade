public class MockSoapService {
 public class EchoRequest {
  public String value;
  private String[] value_type_info=new String[]{'value','urn:mock','xsd:string','0','1','false'};
  private String[] apex_schema_type_info=new String[]{'urn:mock','true','false'};
  private String[] field_order_type_info=new String[]{'value'};
 }
 public class EchoResponse {
  public String value;
  private String[] value_type_info=new String[]{'value','urn:mock','xsd:string','0','1','false'};
  private String[] apex_schema_type_info=new String[]{'urn:mock','true','false'};
  private String[] field_order_type_info=new String[]{'value'};
 }
 public String call(String value) {
  EchoRequest request=new EchoRequest(); request.value=value;
  Map<String,Object> response=new Map<String,Object>();
  String[] info=new String[]{'https://mock.invalid/soap','urn:mock','urn:mock','Echo','urn:mock','EchoResponse','MockSoapService.EchoResponse'};
  System.WebServiceCallout.invoke(this,request,response,info);
  EchoResponse parsed=(EchoResponse)response.get('response_x');
  return parsed.value;
 }
}
