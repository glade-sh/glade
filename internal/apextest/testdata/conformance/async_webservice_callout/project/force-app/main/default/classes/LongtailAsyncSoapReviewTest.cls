@IsTest
private class LongtailAsyncSoapReviewTest {
    static Integer calls = 0;
    class ConstructedFuture extends WebServiceCalloutFuture {
        public ConstructedFuture() { calls++; }
    }
    @IsTest static void begin_constructedFuture() {
        Test.setMock(WebServiceMock.class, new CountingMock());
        System.Continuation c = new System.Continuation(60);
        Object future = WebServiceCallout.beginInvoke(new LongtailAsyncSoapService(), new LongtailAsyncSoapService.EchoRequest(), ConstructedFuture.class, c, new String[]{'https://async-soap.invalid/endpoint','urn:glade:echo','urn:glade:echo','Echo','urn:glade:echo','EchoResponse','LongtailAsyncSoapService.EchoResponse'});
        System.assertEquals('OK|nonnull=true|calls=1', 'OK|nonnull=' + (future != null) + '|calls=' + calls);
    }
    @IsTest static void begin_requestFields() {
        System.Continuation c = new System.Continuation(60);
        new LongtailAsyncSoapService().beginEcho(c, 'request-value');
        HttpRequest request = c.getRequests().values()[0];
        System.assertEquals('method=null|endpoint=null|action=null|body=', 'method=' + request.getMethod() + '|endpoint=' + request.getEndpoint() + '|action=' + request.getHeader('SOAPAction') + '|body=' + request.getBody());
    }
    class CountingMock implements WebServiceMock {
        public void doInvoke(Object stub, Object request, Map<String,Object> response, String endpoint, String soapAction, String requestName, String responseNS, String responseName, String responseType) {
            calls++;
            LongtailAsyncSoapService.EchoResponse value = new LongtailAsyncSoapService.EchoResponse();
            value.value = 'map-response';
            response.put('response_x', value);
        }
    }
    static String observeBegin(String mode) {
        calls = 0;
        if (mode != 'noMock') Test.setMock(WebServiceMock.class, new CountingMock());
        Object stub = new LongtailAsyncSoapService();
        Object request = new LongtailAsyncSoapService.EchoRequest();
        Object futureType = LongtailAsyncSoapService.EchoFuture.class;
        System.Continuation continuation = new System.Continuation(60);
        Object context = continuation;
        String[] info = new String[]{'https://async-soap.invalid/endpoint', 'urn:glade:echo', 'urn:glade:echo', 'Echo', 'urn:glade:echo', 'EchoResponse', 'LongtailAsyncSoapService.EchoResponse'};
        if (mode == 'nullOptions') info = null;
        if (mode == 'sixOptions') info.remove(6);
        if (mode == 'eightOptions') info.add('extra');
        if (mode == 'nullStub') stub = null;
        if (mode == 'nullRequest') request = null;
        if (mode == 'nullContinuation') context = null;
        if (mode == 'wrongContinuation') context = 'wrong';
        if (mode == 'nullType') futureType = null;
        if (mode == 'wrongType') futureType = String.class;
        if (mode == 'dirtyDML') insert new Account(Name='Synthetic async SOAP row');
        Integer before = Limits.getCallouts();
        try {
            Object future = WebServiceCallout.beginInvoke(stub, request, futureType, context, info);
            return 'OK|nonnull=' + (future != null) + '|requests=' + continuation.getRequests().size() + '|calls=' + calls + '|callouts=' + (Limits.getCallouts()-before);
        } catch (Exception e) { return e.getTypeName() + '|' + e.getMessage() + '|calls=' + calls; }
    }
    static String observeEnd(String mode) {
        calls = 0;
        Test.setMock(WebServiceMock.class, new CountingMock());
        LongtailAsyncSoapController controller = new LongtailAsyncSoapController();
        System.Continuation continuation = controller.startRequest();
        String label = continuation.getRequests().keySet().iterator().next();
        if (mode == 'nullFuture' || mode == 'plainFuture' || mode == 'wrongFuture') {
            Object future = null;
            if (mode == 'plainFuture') future = new LongtailAsyncSoapService.EchoFuture();
            if (mode == 'wrongFuture') future = 'wrong';
            try { Object value = WebServiceCallout.endInvoke(future); return 'OK|' + value; }
            catch (Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
        }
        if (mode != 'noResponse') {
            String value = '<m:value>mock-response</m:value>';
            if (mode == 'missingValue') value = '';
            if (mode == 'emptyValue') value = '<m:value/>';
            if (mode == 'escapedValue') value = '<m:value> a &amp; b </m:value>';
            if (mode == 'duplicateValue') value = '<m:value>first</m:value><m:value>second</m:value>';
            String body = '<SOAP:Envelope xmlns:SOAP="http://schemas.xmlsoap.org/soap/envelope/"><SOAP:Body><m:EchoResponse xmlns:m="urn:glade:echo">' + value + '</m:EchoResponse></SOAP:Body></SOAP:Envelope>';
            if (mode == 'emptyBody') body = '';
            if (mode == 'malformed') body = '<broken>';
            if (mode == 'wrongNamespace') body = body.replace('urn:glade:echo','urn:other');
            if (mode == 'wrongFieldNamespace') body = body.replace('<m:value>', '<x:value xmlns:x="urn:other">').replace('</m:value>', '</x:value>');
            if (mode == 'unexpectedField') body = body.replace('m:value','m:extra');
            if (mode == 'nilValue') body = body.replace('<m:value>mock-response</m:value>', '<m:value xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>');
            HttpResponse response = new HttpResponse(); response.setBody(body);
            Test.setContinuationResponse(label,response);
        }
        try { Test.invokeContinuationMethod(controller,continuation); return 'OK|' + controller.result + '|calls=' + calls; }
        catch (Exception e) { return e.getTypeName() + '|' + e.getMessage() + '|calls=' + calls; }
    }
    @IsTest static void begin_dirtyDML() { System.assertEquals('System.CalloutException|You have uncommitted work pending. Please commit or rollback before calling out|calls=0', observeBegin('dirtyDML')); }
    @IsTest static void begin_normal() { System.assertEquals('OK|nonnull=true|requests=1|calls=0|callouts=1', observeBegin('normal')); }
    @IsTest static void begin_noMock() { System.assertEquals('OK|nonnull=true|requests=1|calls=0|callouts=1', observeBegin('noMock')); }
    @IsTest static void begin_nullOptions() { System.assertEquals('System.NullPointerException|Argument 5 cannot be null|calls=0', observeBegin('nullOptions')); }
    @IsTest static void begin_sixOptions() { System.assertEquals('System.TypeException|Invalid info with length 6|calls=0', observeBegin('sixOptions')); }
    @IsTest static void begin_eightOptions() { System.assertEquals('System.TypeException|Invalid info with length 8|calls=0', observeBegin('eightOptions')); }
    @IsTest static void begin_nullStub() { System.assertEquals('System.CalloutException|Web service callout failed: Callout stub not found: null|calls=0', observeBegin('nullStub')); }
    @IsTest static void begin_nullRequest() { System.assertEquals('OK|nonnull=true|requests=1|calls=0|callouts=1', observeBegin('nullRequest')); }
    @IsTest static void begin_nullContinuation() { System.assertEquals('System.NullPointerException|Argument 4 cannot be null|calls=0', observeBegin('nullContinuation')); }
    @IsTest static void begin_nullType() { System.assertEquals('System.NullPointerException|Argument 3 cannot be null|calls=0', observeBegin('nullType')); }
    @IsTest static void begin_wrongType() { System.assertEquals('System.CalloutException| Incompatible return type specified.|calls=0', observeBegin('wrongType')); }
    @IsTest static void end_wrongFieldNamespace() { System.assertEquals('System.CalloutException|Web service callout failed: Unable to parse callout response. Apex type not found for element urn:other=value|calls=0', observeEnd('wrongFieldNamespace')); }
    @IsTest static void end_unexpectedField() { System.assertEquals('System.CalloutException|Web service callout failed: Unable to parse callout response. Apex type not found for element extra|calls=0', observeEnd('unexpectedField')); }
    @IsTest static void end_nilValue() { System.assertEquals('OK|null|calls=0', observeEnd('nilValue')); }
    @IsTest static void end_normal() { System.assertEquals('OK|mock-response|calls=0', observeEnd('normal')); }
    @IsTest static void end_noResponse() { System.assertEquals('System.NullPointerException|Attempt to de-reference a null object|calls=0', observeEnd('noResponse')); }
    @IsTest static void end_emptyBody() { System.assertEquals('System.CalloutException|IO Exception: input contained no data|calls=0', observeEnd('emptyBody')); }
    @IsTest static void end_malformed() { System.assertEquals('System.CalloutException|Web service callout failed: Unexpected element. Parser was expecting element \'http://schemas.xmlsoap.org/soap/envelope/:Envelope\' but found \':broken\'|calls=0', observeEnd('malformed')); }
    @IsTest static void end_missingValue() { System.assertEquals('OK|null|calls=0', observeEnd('missingValue')); }
    @IsTest static void end_emptyValue() { System.assertEquals('OK||calls=0', observeEnd('emptyValue')); }
    @IsTest static void end_escapedValue() { System.assertEquals('OK| a & b |calls=0', observeEnd('escapedValue')); }
    @IsTest static void end_wrongNamespace() { System.assertEquals('System.CalloutException|Web service callout failed: Unexpected element. Parser was expecting element \'urn:glade:echo:EchoResponse\' but found \'urn:other:EchoResponse\'|calls=0', observeEnd('wrongNamespace')); }
    @IsTest static void end_duplicateValue() { System.assertEquals('OK|second|calls=0', observeEnd('duplicateValue')); }
    @IsTest static void end_nullFuture() { System.assertEquals('System.NullPointerException|Argument 1 cannot be null', observeEnd('nullFuture')); }
    @IsTest static void end_plainFuture() { System.assertEquals('OK|null', observeEnd('plainFuture')); }
}
