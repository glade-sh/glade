public class SproutErrorAction {
    public class Request {
        @InvocableVariable public Account record;
        @InvocableVariable public Boolean failMode;
    }
    public class Response { @InvocableVariable public Account record; }
    public class SproutException extends Exception {}
    @InvocableMethod public static List<Response> apply(List<Request> requests) {
        List<Response> outputs = new List<Response>();
        for (Request request : requests) {
            if (request.failMode == true) { throw new SproutException('Sprout fault'); }
            request.record.addError('Sprout record error');
            Response response = new Response();
            response.record = request.record;
            outputs.add(response);
        }
        return outputs;
    }
}
