@IsTest
private class Mx_assignmentFirst {
    private static String obs(String variant, Exception e, String saveResults, String recId) {
        Boolean perm = FeatureManagement.checkPermission('PsrPerm');
        String fls = recId == null ? 'noRec' : PsrUtil.flsRead(recId);
        Boolean later = false;
        String laterFls = '';
        if (recId != null) {
            System.runAs(new User(Id = UserInfo.getUserId())) {
                later = FeatureManagement.checkPermission('PsrPerm');
                laterFls = PsrUtil.flsRead(recId);
            }
        }
        Integer n = [SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId = :UserInfo.getUserId()];
        return 'variant=' + variant + ';perm=' + perm + ';fls=' + fls +
            ';runAsSelf_perm=' + later + ';runAsSelf_fls=' + laterFls +
            ';psaRows=' + n +
            ';saveResults=' + saveResults +
            ';ex=' + (e == null ? '' : e.getTypeName() + ':' + e.getMessage());
    }

    private static String results(Database.SaveResult[] rs) {
        if (rs == null) {
            return 'null';
        }
        String out = '';
        for (Database.SaveResult r : rs) {
            out += (r.isSuccess() ? 'ok' : 'err:' + r.getErrors()[0].getStatusCode() + ':' + r.getErrors()[0].getMessage()) + '|';
        }
        return out;
    }

    private static String findRec(String name) {
        List<PsrObj__c> rows = [SELECT Id FROM PsrObj__c WHERE Name = :name LIMIT 1];
        return rows.isEmpty() ? null : rows[0].Id;
    }

    @IsTest
    static void assignmentFirst() {
        Id psId = PsrUtil.permSetId();
        List<SObject> items = new List<SObject>();
        items.add(new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId));
        items.add(new PsrObj__c(Name = 'mx1', PsrField__c = 'v'));
        items.add(new PsrObj__c(Name = 'mx2', PsrField__c = 'v'));
        Exception ex;
        try {
            insert items;
        } catch (Exception e) {
            ex = e;
        }
        System.assert(false, obs('assignmentFirst', ex, 'n/a', findRec('mx1')));
    }
}
