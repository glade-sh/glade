@IsTest
private class PsrVersionTest {
    @IsTest
    static void requestVersionProbe() {
        String outv;
        try {
            System.Version v = System.requestVersion();
            outv = 'ok:' + String.valueOf(v);
        } catch (Exception e) {
            outv = 'ex:' + e.getTypeName() + ':' + e.getMessage();
        }
        System.assert(false, 'probe=requestVersion;value=' + outv);
    }

    @IsTest
    static void nestedVersionThenSelf() {
        Id psId = PsrUtil.permSetId();
        System.Version v = new System.Version(40, 0);
        String outerVal = 'notRun';
        String innerVal = 'notRun';
        Exception ex;
        try {
            System.runAs(v) {
                outerVal = String.valueOf(FeatureManagement.checkPermission('PsrPerm'));
                System.runAs(new User(Id = UserInfo.getUserId())) {
                    try {
                        insert new PermissionSetAssignment(
                            AssigneeId = UserInfo.getUserId(), PermissionSetId = psId
                        );
                        innerVal = String.valueOf(FeatureManagement.checkPermission('PsrPerm'));
                    } catch (Exception e2) {
                        innerVal = 'insEX:' + e2.getTypeName() + ':' + e2.getMessage();
                    }
                }
            }
        } catch (Exception e) {
            ex = e;
        }
        System.assert(false,
            'shape=nestedVersionThenSelf;version=' + String.valueOf(v) +
            ';outerRead=' + outerVal + ';innerVal=' + innerVal +
            ';ex=' + (ex == null ? '' : ex.getTypeName() + ':' + ex.getMessage())
        );
    }

    @IsTest
    static void innerOnly() {
        Id psId = PsrUtil.permSetId();
        String innerVal = 'notRun';
        try {
            System.runAs(new User(Id = UserInfo.getUserId())) {
                insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId);
                innerVal = String.valueOf(FeatureManagement.checkPermission('PsrPerm'));
            }
        } catch (Exception e) {
            innerVal = 'EX:' + e.getTypeName() + ':' + e.getMessage();
        }
        System.assert(false, 'shape=innerOnlySelfRunAs;innerVal=' + innerVal);
    }

    @IsTest
    static void plainVersionRead() {
        Id psId = PsrUtil.permSetId();
        String ins = 'ok';
        try {
            insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId);
        } catch (Exception e) {
            ins = 'ex:' + e.getTypeName();
        }
        System.Version v = new System.Version(40, 0);
        String inside = 'notRun';
        Exception ex;
        try {
            System.runAs(v) {
                inside = String.valueOf(FeatureManagement.checkPermission('PsrPerm'));
            }
        } catch (Exception e) {
            ex = e;
        }
        System.assert(false,
            'shape=plainVersionReadAfterAssign;version=' + String.valueOf(v) +
            ';insert=' + ins + ';inside=' + inside +
            ';ex=' + (ex == null ? '' : ex.getTypeName() + ':' + ex.getMessage())
        );
    }

    @IsTest
    static void plainVersionAssign() {
        Id psId = PsrUtil.permSetId();
        System.Version v = new System.Version(40, 0);
        String inside = 'notRun';
        Exception ex;
        try {
            System.runAs(v) {
                try {
                    insert new PermissionSetAssignment(
                        AssigneeId = UserInfo.getUserId(), PermissionSetId = psId
                    );
                    inside = String.valueOf(FeatureManagement.checkPermission('PsrPerm'));
                } catch (Exception e2) {
                    inside = 'insEX:' + e2.getTypeName() + ':' + e2.getMessage();
                }
            }
        } catch (Exception e) {
            ex = e;
        }
        System.assert(false,
            'shape=plainVersionAssignInside;version=' + String.valueOf(v) +
            ';inside=' + inside +
            ';ex=' + (ex == null ? '' : ex.getTypeName() + ':' + ex.getMessage())
        );
    }
}
