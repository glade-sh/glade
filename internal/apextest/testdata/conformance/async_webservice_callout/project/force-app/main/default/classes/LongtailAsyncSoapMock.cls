public class LongtailAsyncSoapMock implements WebServiceMock {
    public void doInvoke(
        Object stub,
        Object request,
        Map<String,Object> response,
        String endpoint,
        String soapAction,
        String requestName,
        String responseNS,
        String responseName,
        String responseType
    ) {
        // The continuation response is supplied through Test.setContinuationResponse.
    }
}
