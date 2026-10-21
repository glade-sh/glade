public class ImmediateEventFlowReceiver {
    @InvocableMethod(label='Record owned immediate events')
    public static void record(List<ImmediateFlowNotice__e> events) {
        List<Account> records = new List<Account>();
        for (ImmediateFlowNotice__e event : events) records.add(new Account(Name=event.Token__c));
        insert records;
    }
}
