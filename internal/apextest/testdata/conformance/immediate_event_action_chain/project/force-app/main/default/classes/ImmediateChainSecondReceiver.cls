public class ImmediateChainSecondReceiver {
    @InvocableMethod(label='Record second owned action')
    public static void receive(List<ImmediateChainNotice__e> events) {
        List<Account> records = new List<Account>();
        for (ImmediateChainNotice__e event : events) {
            String firstName = event.Token__c+'-first';
            Integer priorCount = [SELECT COUNT() FROM Account WHERE Name=:firstName];
            records.add(new Account(Name=event.Token__c+'-second', Description='first count='+priorCount));
        }
        insert records;
    }
}
