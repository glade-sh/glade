public class PersonBehavior {
    public static String personDefault() {
        Id rt = Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('PersonAccount').getRecordTypeId();
        Account a = new Account(RecordTypeId=rt, LastName='Test1', PersonEmail='shared@sample.example');
        insert a;
        StandardBehavior.remember(a.Id, 'first');
        Account b = new Account(RecordTypeId=rt, LastName='Test2', PersonEmail=a.PersonEmail);
        return StandardBehavior.find(b) + '|' + StandardBehavior.save(Database.insert(b, false)) + '|count=' + [SELECT COUNT() FROM Account WHERE IsPersonAccount=true AND PersonEmail='shared@sample.example'];
    }
}
