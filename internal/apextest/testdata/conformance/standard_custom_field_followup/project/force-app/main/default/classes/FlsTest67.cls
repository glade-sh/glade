@IsTest private class FlsTest67 {
    @IsTest static void queryOtherName() {
        String result;
        try { result = 'ok:' + Database.query('SELECT Probe_B__c FROM Account WHERE Name = \'y\'').size(); }
        catch (Exception e) { result = e.getTypeName() + ':' + e.getMessage(); }
        System.assert(false, result);
    }
    @IsTest static void failedSetupThenQuery() {
        Id ps = [SELECT Id FROM PermissionSet WHERE Name = 'Probe_Access'].Id;
        insert new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(), PermissionSetId=ps);
        try { insert new PermissionSet(); } catch (Exception e) {}
        String result;
        try { result = 'ok:' + Database.query('SELECT Probe_A__c FROM Account LIMIT 1').size(); }
        catch (Exception e) { result = e.getTypeName() + ':' + e.getMessage(); }
        System.assert(false, result);
    }
    @IsTest static void capturedNameControl() {
        String result;
        try { result = 'ok:' + Database.query('SELECT Probe_B__c FROM Account WHERE Name = \'x\'').size(); }
        catch (Exception e) { result = e.getTypeName() + ':' + e.getMessage(); }
        System.assert(false, result);
    }
    @IsTest static void otherAssignment() {
        Id ps = [SELECT Id FROM PermissionSet WHERE Name = 'Second_Access'].Id;
        insert new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(), PermissionSetId=ps);
        String result;
        try { result = 'ok:' + Database.query('SELECT Probe_A__c FROM Account LIMIT 1').size(); }
        catch (Exception e) { result = e.getTypeName() + ':' + e.getMessage(); }
        System.assert(false, result);
    }
}
