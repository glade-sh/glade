@IsTest(SeeAllData=true)
private class CapShare67Test {
    private static Id groupId() {
        return [SELECT Id FROM Group WHERE DeveloperName = 'CapShareGroup' LIMIT 1].Id;
    }
    private static Id newOwner() {
        return [SELECT Id FROM User WHERE Alias = 'cap68u1' LIMIT 1].Id;
    }
    @IsTest static void manualShare() {
        Id recipients = groupId();
        Id nextOwner = newOwner();
        Savepoint sp = Database.setSavepoint();
        try {
            Account a = new Account(Name = 'S68-owner-transfer');
            insert as system a;
            AccountShare s = new AccountShare(AccountId = a.Id, UserOrGroupId = recipients,
                AccountAccessLevel = 'Read', OpportunityAccessLevel = 'None',
                CaseAccessLevel = 'None', RowCause = 'Manual');
            insert as system s;
            a.OwnerId = nextOwner;
            update as system a;
            Integer n = [SELECT COUNT() FROM AccountShare WHERE AccountId = :a.Id
                AND UserOrGroupId = :recipients AND RowCause = 'Manual' WITH SYSTEM_MODE];
            System.debug(LoggingLevel.ERROR, 'MANUAL_SHARES=' + n);
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'MANUAL_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
        Database.rollback(sp);
    }
    @IsTest static void customReason() {
        Id recipients = groupId();
        Savepoint sp = Database.setSavepoint();
        try {
            Account a = new Account(Name = 'S68-custom-reason');
            insert as system a;
            AccountShare s = new AccountShare(AccountId = a.Id, UserOrGroupId = recipients,
                AccountAccessLevel = 'Read', OpportunityAccessLevel = 'None',
                CaseAccessLevel = 'None', RowCause = 'Capture__c');
            insert as system s;
            Integer n = [SELECT COUNT() FROM AccountShare WHERE AccountId = :a.Id
                AND UserOrGroupId = :recipients AND RowCause = 'Capture__c' WITH SYSTEM_MODE];
            System.debug(LoggingLevel.ERROR, 'CUSTOM_SHARES=' + n);
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'CUSTOM_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
        Database.rollback(sp);
    }
    @IsTest static void profileQuery() {
        User limited = [SELECT Id FROM User WHERE Alias = 'cap68u1' LIMIT 1];
        System.runAs(limited) {
            List<String> levelNames = new List<String>{'USER_MODE', 'SYSTEM_MODE'};
            List<AccessLevel> levels = new List<AccessLevel>{
                AccessLevel.USER_MODE, AccessLevel.SYSTEM_MODE};
            for (Integer i = 0; i < levels.size(); i++) {
                try {
                    List<SObject> rows = Database.queryWithBinds(
                        'SELECT Id, Name FROM Profile ORDER BY Id',
                        new Map<String,Object>(), levels[i]);
                    System.debug(LoggingLevel.ERROR, 'MODE=' + levelNames[i] + ';COUNT=' + rows.size()
                        + ';OWN=' + UserInfo.getProfileId());
                } catch (Exception e) {
                    System.debug(LoggingLevel.ERROR, 'MODE=' + levelNames[i]
                        + ';ERR=' + e.getTypeName() + ': ' + e.getMessage());
                }
            }
        }
    }
}
