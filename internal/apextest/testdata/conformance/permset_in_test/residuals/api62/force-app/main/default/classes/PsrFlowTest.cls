@IsTest
private class PsrFlowTest {
    private static String runFlow() {
        try {
            Flow.Interview f = Flow.Interview.createInterview('PsrFlow', new Map<String, Object>());
            f.start();
            return 'ok:' + String.valueOf(f.getVariableValue('Out'));
        } catch (Exception e) {
            return 'ex:' + e.getTypeName() + ':' + e.getMessage();
        }
    }

    @IsTest
    static void beforeAssignment() {
        String before = runFlow();
        System.assert(false, 'phase=before;flow=' + before);
    }

    @IsTest
    static void afterAssignment() {
        Id psId = PsrUtil.permSetId();
        String ins = 'ok';
        try {
            insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = psId);
        } catch (Exception e) {
            ins = 'ex:' + e.getTypeName() + ':' + e.getMessage();
        }
        String after = runFlow();
        String inRunAs;
        System.runAs(new User(Id = UserInfo.getUserId())) {
            inRunAs = runFlow();
        }
        System.assert(false, 'phase=after;insert=' + ins + ';flow=' + after + ';runAsSelf_flow=' + inRunAs);
    }
}
