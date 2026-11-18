@IsTest
private class PsrUraTest {
    @TestSetup
    static void setup() {
        insert new PsrObj__c(Name = 'ura', PsrField__c = 'v');
    }

    @IsTest
    static void userRecordAccessAdminInsert() {
        Id psId = PsrUtil.permSetId();
        Id recId = [SELECT Id FROM PsrObj__c WHERE Name = 'ura' LIMIT 1].Id;
        User u = PsrUtil.newUser(PsrUtil.limitedProfileId(), 'UR', 1);
        insert u;

        String before = PsrUtil.ura(u.Id, recId);
        String ins = 'ok';
        try {
            insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = psId);
        } catch (Exception e) {
            ins = 'ex:' + e.getTypeName() + ':' + e.getMessage();
        }
        String afterAsAdmin = PsrUtil.ura(u.Id, recId);
        String afterRunAsU;
        String permInU;
        System.runAs(u) {
            afterRunAsU = PsrUtil.ura(u.Id, recId);
            permInU = String.valueOf(FeatureManagement.checkPermission('PsrPerm'));
        }
        System.assert(false,
            'before=' + before +
            ';insert=' + ins +
            ';afterAsAdmin=' + afterAsAdmin +
            ';afterRunAsU=' + afterRunAsU +
            ';permInU=' + permInU
        );
    }

    @IsTest
    static void userRecordAccessInsideBlock() {
        Id psId = PsrUtil.permSetId();
        Id recId = [SELECT Id FROM PsrObj__c WHERE Name = 'ura' LIMIT 1].Id;
        User u = PsrUtil.newUser(PsrUtil.limitedProfileId(), 'UR', 2);
        insert u;

        String before;
        String after;
        String ins = 'ok';
        String permAfter;
        System.runAs(u) {
            before = PsrUtil.ura(u.Id, recId);
            try {
                insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = psId);
            } catch (Exception e) {
                ins = 'ex:' + e.getTypeName() + ':' + e.getMessage();
            }
            after = PsrUtil.ura(u.Id, recId);
            permAfter = String.valueOf(FeatureManagement.checkPermission('PsrPerm'));
        }
        String afterOuter = PsrUtil.ura(u.Id, recId);
        String permOuter;
        System.runAs(u) {
            permOuter = String.valueOf(FeatureManagement.checkPermission('PsrPerm'));
        }
        System.assert(false,
            'before=' + before + ';insert=' + ins + ';after=' + after +
            ';permAfter=' + permAfter + ';afterOuter=' + afterOuter + ';permOuter=' + permOuter
        );
    }
}
