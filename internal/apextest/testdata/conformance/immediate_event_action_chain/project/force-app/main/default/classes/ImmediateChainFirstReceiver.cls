public class ImmediateChainFirstReceiver {
    @InvocableMethod(label='Record first owned action')
    public static void receive(List<ImmediateChainNotice__e> events) {
        List<Account> records = new List<Account>();
        for (ImmediateChainNotice__e event : events) {
            records.add(new Account(Name=event.Token__c+'-first', Description='first action'));
        }
        insert records;
    }
}
