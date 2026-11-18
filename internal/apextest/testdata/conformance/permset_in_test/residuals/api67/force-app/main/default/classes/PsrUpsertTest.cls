@IsTest
private class PsrUpsertTest {
    @TestSetup
    static void setup() {
        insert as system new PsrObj__c(Name = 'up', PsrField__c = 'v');
    }

    private static String observe(String op, Exception e, Boolean inRunAs) {
        Id recId = [SELECT Id FROM PsrObj__c WHERE Name = 'up' LIMIT 1].Id;
        Boolean perm = FeatureManagement.checkPermission('PsrPerm');
        String fls = PsrUtil.flsRead(recId);
        Boolean later = false;
        String laterFls = '';
        System.runAs(new User(Id = UserInfo.getUserId())) {
            later = FeatureManagement.checkPermission('PsrPerm');
            laterFls = PsrUtil.flsRead(recId);
        }
        return 'op=' + op + ';perm=' + perm + ';fls=' + fls +
            ';runAsSelf_perm=' + later + ';runAsSelf_fls=' + laterFls +
            ';ex=' + (e == null ? '' : e.getTypeName() + ':' + e.getMessage());
    }

    @IsTest
    static void upsertPlain() {
        Id psId = PsrUtil.permSetId();
        Exception ex;
        try {
            upsert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId);
        } catch (Exception e) {
            ex = e;
        }
        System.assert(false, observe('upsert', ex, false));
    }

    @IsTest
    static void upsertDatabase() {
        Id psId = PsrUtil.permSetId();
        Exception ex;
        try {
            Database.upsert(
                new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId), false
            );
        } catch (Exception e) {
            ex = e;
        }
        System.assert(false, observe('Database.upsert', ex, false));
    }

    @IsTest
    static void insertPlain() {
        Id psId = PsrUtil.permSetId();
        Exception ex;
        try {
            insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId);
        } catch (Exception e) {
            ex = e;
        }
        System.assert(false, observe('insert', ex, false));
    }

    @IsTest
    static void insertDatabase() {
        Id psId = PsrUtil.permSetId();
        Exception ex;
        try {
            Database.insert(
                new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId), false
            );
        } catch (Exception e) {
            ex = e;
        }
        System.assert(false, observe('Database.insert', ex, false));
    }

    @IsTest
    static void upsertWithId() {
        Id psId = PsrUtil.permSetId();
        PermissionSetAssignment psa = new PermissionSetAssignment(
            AssigneeId = UserInfo.getUserId(), PermissionSetId = psId
        );
        insert psa;
        Exception ex;
        try {
            upsert psa;
        } catch (Exception e) {
            ex = e;
        }
        System.assert(false, observe('upsertExistingId', ex, false));
    }
}
