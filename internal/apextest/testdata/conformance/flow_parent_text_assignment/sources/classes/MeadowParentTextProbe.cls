public class MeadowParentTextProbe {
    public static void seed() {
        Database.insert(new List<Account>{
            new Account(Name='Meadow alpha', MeadowText__c='Marker Alpha'),
            new Account(Name='Meadow beta', MeadowText__c='Marker Beta'),
            new Account(Name='Meadow empty', MeadowText__c=null)
        }, AccessLevel.SYSTEM_MODE);
    }

    public static Map<String, Object> observe() {
        Savepoint point = Database.setSavepoint();
        try {
            Map<String, Id> parents = new Map<String, Id>();
            for (Account row : [SELECT Id, Name FROM Account WHERE Name IN ('Meadow alpha','Meadow beta','Meadow empty')]) {
                parents.put(row.Name, row.Id);
            }
            Map<String, Object> answers = new Map<String, Object>();
            answers.put('P01', capture(new Contact(LastName='Meadow populated', MeadowParent__c=parents.get('Meadow alpha'), Description='Initial', Email='populated@example.invalid'), null));
            answers.put('P02', capture(new Contact(LastName='Meadow absent', MeadowParent__c=null, Description='Initial', Email='absent@example.invalid'), null));
            answers.put('P03', capture(new Contact(LastName='Meadow empty text', MeadowParent__c=parents.get('Meadow empty'), Description='Initial', Email='empty@example.invalid'), null));
            answers.put('P04', capture(new Contact(LastName='Meadow reparent', MeadowParent__c=parents.get('Meadow alpha'), Description='Initial', Email='reparent@example.invalid'), parents.get('Meadow beta')));
            answers.put('P05', capture(new Contact(LastName='Meadow direct', MeadowParent__c=parents.get('Meadow alpha'), Description='Initial', Email='direct@example.invalid'), null));
            return answers;
        } finally {
            Database.rollback(point);
        }
    }

    private static Map<String, Object> capture(Contact row, Id replacementParent) {
        try {
            Database.insert(row, AccessLevel.SYSTEM_MODE);
            if (replacementParent != null) {
                row.MeadowParent__c = replacementParent;
                Database.update(row, AccessLevel.SYSTEM_MODE);
            }
            Contact stored = [SELECT Description, Email FROM Contact WHERE Id=:row.Id];
            return new Map<String, Object>{'description'=>stored.Description, 'email'=>stored.Email, 'exception'=>null};
        } catch (Exception problem) {
            return new Map<String, Object>{'description'=>null, 'email'=>null, 'exception'=>problem.getTypeName()};
        }
    }
}
