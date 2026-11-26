for (EntityDefinition e : [SELECT QualifiedApiName, InternalSharingModel, ExternalSharingModel FROM EntityDefinition WHERE QualifiedApiName IN ('Case','Lead') ORDER BY QualifiedApiName]) {
    System.debug('ROADOBS|models|' + e.QualifiedApiName + '|' + e.InternalSharingModel + '|' + e.ExternalSharingModel);
}
List<Id> records = new List<Id>{
    [SELECT Id FROM Case WHERE Subject='Sharing Probe Other' LIMIT 1].Id,
    [SELECT Id FROM Case WHERE Subject='Sharing Probe Contact Case' LIMIT 1].Id,
    [SELECT Id FROM Lead WHERE LastName='Sharing Probe Lead' LIMIT 1].Id
};
for (User u : [SELECT Id, Alias, UserType FROM User WHERE Alias IN ('rdstd','rdcc') OR UserType='Guest' ORDER BY Alias]) {
    String outcome = '';
    for (Id recordId : records) {
        List<UserRecordAccess> access = [SELECT RecordId, HasReadAccess, HasEditAccess FROM UserRecordAccess WHERE UserId = :u.Id AND RecordId = :recordId];
        outcome += '|' + (access.isEmpty() ? '-' : (access[0].HasReadAccess ? '1' : '0') + (access[0].HasEditAccess ? '1' : '0'));
    }
    System.debug('ROADOBS|access|' + (u.UserType == 'Guest' ? 'guest' : u.Alias) + outcome);
}
