public class ExtraBehavior {
    private static String upsertExternal(Boolean matchedKey) {
        Account a = new Account(Name='Test Account 1', BillingCity='Northport');
        Account b = new Account(Name='Elsewhere', BillingCity='Distant', ProbeKey__c='stored');
        insert new List<Account>{a,b};
        StandardBehavior.remember(a.Id, 'first');
        Account c = new Account(Name='Test Account 2', BillingCity='Northport', ProbeKey__c=matchedKey ? 'stored' : 'new');
        Database.UpsertResult r = Database.upsert(c, Account.ProbeKey__c, false);
        return StandardBehavior.verdict(r.isSuccess(),r.getErrors()) + '|count=' + [SELECT COUNT() FROM Account WHERE BillingCity='Northport'];
    }
    public static String externalNew() { return upsertExternal(false); }
    public static String externalExisting() { return upsertExternal(true); }
    private static String namePair(String first, String last, String first2, String last2) {
        Contact a = new Contact(FirstName=first,LastName=last,Email='shared@sample.example');
        insert a;
        StandardBehavior.remember(a.Id,'first');
        Contact b = new Contact(FirstName=first2,LastName=last2,Email=a.Email);
        return StandardBehavior.find(b) + '|' + StandardBehavior.save(Database.insert(b,false));
    }
    public static String initialForward() { return namePair('J','Hartwell','Jonathan','Hartwell'); }
    public static String initialReverse() { return namePair('Jonathan','Hartwell','J','Hartwell'); }
    public static String transposedLast() { return namePair('Jonathan','Smiht','Jonathan','Smith'); }
}
