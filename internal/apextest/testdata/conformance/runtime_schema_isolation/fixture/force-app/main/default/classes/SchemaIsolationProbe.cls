public class SchemaIsolationProbe {
    public static String observe() {
        Integer beforeCount = [SELECT COUNT() FROM Account WHERE Name = 'Owned transient'];
        Account parent = new Account(Name = 'Owned transient');
        insert parent;
        insert new Contact(LastName = 'Owned child', AccountId = parent.Id);
        parent = [SELECT Id, Name, (SELECT LastName FROM Contacts) FROM Account WHERE Id = :parent.Id];
        Account decoded = (Account)JSON.deserialize(JSON.serialize(parent), Account.class);
        String fieldName = Account.Name.getDescribe().getName();
        return beforeCount + '|' + parent.Name + '|' + decoded.Contacts.size() + '|' + fieldName;
    }
}
