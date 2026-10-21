@IsTest(SeeAllData=true) private without sharing class RecordAccessModeProbe {
    @IsTest static void observe() {
        User actor = [SELECT Id FROM User WHERE Username='synthetic.record.access.20261019@example.invalid' LIMIT 1];
        Account target = [SELECT Id FROM Account WHERE Name='Synthetic record access target' LIMIT 1];
        Id actorId = actor.Id;
        Id targetId = target.Id;
        System.runAs(actor) {
            UserRecordAccess implicit = [SELECT RecordId, HasReadAccess FROM UserRecordAccess WHERE UserId=:actorId AND RecordId=:targetId];
            System.assertEquals(true, implicit.HasReadAccess, 'implicit static query should return the captured access');
            List<SObject> explicitRows = Database.query('SELECT RecordId, HasReadAccess FROM UserRecordAccess WHERE UserId=\'' + actorId + '\' AND RecordId=\'' + targetId + '\'', AccessLevel.USER_MODE);
            System.assertEquals(1, explicitRows.size(), 'USER_MODE query should return the captured access row');
            UserRecordAccess explicitAccess = (UserRecordAccess)explicitRows[0];
            System.assertEquals(true, explicitAccess.HasReadAccess, 'USER_MODE query should expose the captured access flag');
        }
    }
}
