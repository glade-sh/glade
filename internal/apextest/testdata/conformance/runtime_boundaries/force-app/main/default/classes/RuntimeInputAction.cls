public class RuntimeInputAction {
    public class Request {
        @InvocableVariable public String accountId;
        @InvocableVariable public String text = 'seed';
        @InvocableVariable public String mode = 'initial';
        @InvocableVariable public Integer count = 7;
        @InvocableVariable public Datetime timestamp = System.now();
        public Request() { count = 11; }
    }
    @InvocableMethod public static void invoke(List<Request> requests) {
        for (Request r : requests) {
            update new Account(Id = r.accountId, Description = r.text + '|' + r.count + '|' + r.mode + '|' + (r.timestamp != null));
        }
    }
}
