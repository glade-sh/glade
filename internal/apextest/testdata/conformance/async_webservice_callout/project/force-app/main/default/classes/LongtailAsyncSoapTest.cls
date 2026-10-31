@IsTest
private class LongtailAsyncSoapTest {
    @IsTest
    static void callbackReturnsMockedResponse() {
        Test.setMock(WebServiceMock.class, new LongtailAsyncSoapMock());
        LongtailAsyncSoapController controller = new LongtailAsyncSoapController();
        System.Continuation continuation = controller.startRequest();

        Map<String, HttpRequest> requests = continuation.getRequests();
        System.assertEquals(1, requests.size());
        HttpResponse response = new HttpResponse();
        response.setBody(
            '<SOAP:Envelope xmlns:SOAP="http://schemas.xmlsoap.org/soap/envelope/">' +
            '<SOAP:Body><m:EchoResponse xmlns:m="urn:glade:echo">' +
            '<m:value>mock-response</m:value></m:EchoResponse></SOAP:Body></SOAP:Envelope>'
        );
        String requestLabel = requests.keySet().iterator().next();
        Test.setContinuationResponse(requestLabel, response);
        Test.invokeContinuationMethod(controller, continuation);
        System.assertEquals('mock-response', controller.result);
    }

    @IsTest
    static void sevenSoapOptionsAreAccepted() {
        Test.setMock(WebServiceMock.class, new LongtailAsyncSoapMock());
        System.Continuation continuation = new System.Continuation(60);
        continuation.continuationMethod = 'processResponse';
        LongtailAsyncSoapService service = new LongtailAsyncSoapService();
        LongtailAsyncSoapService.EchoRequest request = new LongtailAsyncSoapService.EchoRequest();
        Object future = System.WebServiceCallout.beginInvoke(
            service,
            request,
            LongtailAsyncSoapService.EchoFuture.class,
            continuation,
            new String[]{'https://async-soap.invalid/endpoint', 'urn:glade:echo', 'urn:glade:echo', 'Echo', 'urn:glade:echo', 'EchoResponse', 'LongtailAsyncSoapService.EchoResponse'}
        );
        System.assertNotEquals(null, future);
    }

    @IsTest
    static void eightSoapOptionsAreRejected() {
        Test.setMock(WebServiceMock.class, new LongtailAsyncSoapMock());
        System.Continuation continuation = new System.Continuation(60);
        continuation.continuationMethod = 'processResponse';
        LongtailAsyncSoapService service = new LongtailAsyncSoapService();
        LongtailAsyncSoapService.EchoRequest request = new LongtailAsyncSoapService.EchoRequest();
        String observation = 'returned';
        try {
            System.WebServiceCallout.beginInvoke(
                service,
                request,
                LongtailAsyncSoapService.EchoFuture.class,
                continuation,
                new String[]{'https://async-soap.invalid/endpoint', 'urn:glade:echo', 'urn:glade:echo', 'Echo', 'urn:glade:echo', 'EchoResponse', 'LongtailAsyncSoapService.EchoResponse', 'extra'}
            );
        } catch (Exception problem) {
            observation = problem.getTypeName() + '|' + problem.getMessage();
        }
        System.assertEquals('System.TypeException|Invalid info with length 8', observation);
    }
}
