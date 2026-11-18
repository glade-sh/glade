@IsTest private class IsolateTest {
    private static User standardUser() {
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        User u = new User(ProfileId=p.Id, Alias='probe', Email='isolation@example.invalid',
            EmailEncodingKey='UTF-8', LastName='Isolation', LanguageLocaleKey='en_US',
            LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles',
            UserName='isolation.'+String.valueOf(System.now().getTime())+'@example.invalid');
        insert u; return u;
    }
    @IsTest static void adminWith() { System.assert(false, 'CAPTURE|'+JSON.serialize(IsolateWith.observe())); }
    @IsTest static void adminWithout() { System.assert(false, 'CAPTURE|'+JSON.serialize(IsolateWithout.observe())); }
    @IsTest static void standardWith() { User u=standardUser(); System.runAs(u) { System.assert(false, 'CAPTURE|'+JSON.serialize(IsolateWith.observe())); } }
    @IsTest static void standardWithout() { User u=standardUser(); System.runAs(u) { System.assert(false, 'CAPTURE|'+JSON.serialize(IsolateWithout.observe())); } }
}
