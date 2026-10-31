public class LongtailAsyncSoapController {
    public LongtailAsyncSoapService.EchoFuture future;
    public String result;

    public System.Continuation startRequest() {
        System.Continuation continuation = new System.Continuation(60);
        continuation.continuationMethod = 'processResponse';
        future = new LongtailAsyncSoapService().beginEcho(continuation, 'request-value');
        return continuation;
    }

    public Object processResponse() {
        result = future.getValue();
        return null;
    }
}
