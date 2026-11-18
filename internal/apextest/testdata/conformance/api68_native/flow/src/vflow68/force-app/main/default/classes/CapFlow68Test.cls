@IsTest(SeeAllData=true)
private class CapFlow68Test {
    @IsTest static void limitedUser() {
        User limited = [SELECT Id FROM User WHERE Alias = 'cap68u1' LIMIT 1];
        Account a = new Account(Name = 'S68-flow-limited');
        insert as system a;
        System.runAs(limited) {
            CapFlowCaller68.run('CapFlow68', a.Id);
        }
        Account after = [SELECT Description FROM Account WHERE Id = :a.Id WITH SYSTEM_MODE];
        System.debug(LoggingLevel.ERROR, 'DESCRIPTION=' + after.Description);
    }
    @IsTest static void limitedUserDefault68() {
        User limited = [SELECT Id FROM User WHERE Alias = 'cap68u1' LIMIT 1];
        Account a = new Account(Name = 'S68-flow-default68');
        insert as system a;
        System.runAs(limited) {
            CapFlowCaller68.run('CapFlow68Default', a.Id);
        }
        Account after = [SELECT Description FROM Account WHERE Id = :a.Id WITH SYSTEM_MODE];
        System.debug(LoggingLevel.ERROR, 'DEFAULT68_DESCRIPTION=' + after.Description);
    }
    @IsTest static void adminUser() {
        Account a = new Account(Name = 'S68-flow-admin');
        insert as system a;
        CapFlowCaller68.run('CapFlow68', a.Id);
        Account after = [SELECT Description FROM Account WHERE Id = :a.Id WITH SYSTEM_MODE];
        System.debug(LoggingLevel.ERROR, 'ADMIN_DESCRIPTION=' + after.Description);
    }
}
