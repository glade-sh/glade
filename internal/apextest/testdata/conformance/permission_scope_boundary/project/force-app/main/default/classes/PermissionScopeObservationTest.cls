@IsTest
private class PermissionScopeObservationTest {
    @IsTest static void invalidScope() { observe('invalidScope','not-an-id'); }
    @IsTest static void nullScope() { observe('nullScope',null); }
    @IsTest static void missingScope() { observe('missingScope','0PS000000000001'); }
    @IsTest static void baseScope() {
        AccessLevel scope = AccessLevel.USER_MODE;
        emit('baseScope',new Map<String,Object>{'present'=>scope!=null});
    }
    private static void observe(String method, String permissionSetId) {
        Map<String,Object> result = new Map<String,Object>{'accepted'=>false};
        try {
            AccessLevel scope = AccessLevel.USER_MODE.withPermissionSetId(permissionSetId);
            result.put('accepted',true);
            result.put('present',scope!=null);
        } catch (Exception failure) {
            result.put('type',failure.getTypeName());
            result.put('message',failure.getMessage());
        }
        emit(method,result);
    }
    private static void emit(String method, Map<String,Object> result) {
        System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(result));
    }
}
