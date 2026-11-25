public class HarborFlowAction {
    public static Integer calls = 0;
    @InvocableMethod(label='Owned action')
    public static void execute(List<Request> requests) { calls += requests.size(); }
    public class Request {
        @InvocableVariable public String value;
    }
}
