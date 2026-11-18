List<User> us = [SELECT Id FROM User WHERE Alias = 'hid62' AND LastName = 'Hidden' LIMIT 1];
if (us.isEmpty()) {
    Profile p = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
    User u = new User(LastName = 'Hidden', Alias = 'hid62', Email = 'glade.hidden@example.invalid',
        Username = 'glade.hidden.' + String.valueOf(Crypto.getRandomLong()) + '@example.invalid',
        ProfileId = p.Id, TimeZoneSidKey = 'America/Los_Angeles', LocaleSidKey = 'en_US',
        EmailEncodingKey = 'UTF-8', LanguageLocaleKey = 'en_US');
    insert u;
}
List<User> now = [SELECT Id FROM User WHERE Alias = 'hid62' AND LastName = 'Hidden' LIMIT 1];
Id uid = now[0].Id;
List<PermissionSet> pss = [SELECT Id FROM PermissionSet WHERE Name = 'HidAccess' LIMIT 1];
Id pid = pss[0].Id;
if ([SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId = :uid AND PermissionSetId = :pid] == 0) {
    insert new PermissionSetAssignment(AssigneeId = uid, PermissionSetId = pid);
}
System.debug('OBS|user|u=' + [SELECT COUNT() FROM User WHERE Alias = 'hid62'] + '|p=' + [SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId = :uid]);
