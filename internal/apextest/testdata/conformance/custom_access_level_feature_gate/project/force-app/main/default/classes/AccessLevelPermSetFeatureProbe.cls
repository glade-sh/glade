@IsTest
private class AccessLevelPermSetFeatureProbe {
    @IsTest
    static void observe() {
        Map<String, Object> observation = new Map<String, Object>();
        try {
            Id permissionSetId = Id.valueOf(
                PermissionSet.SObjectType.getDescribe().getKeyPrefix() + '000000000000'
            );
            AccessLevel accessLevel = AccessLevel.USER_MODE.withPermissionSetId(permissionSetId);
            List<SObject> rows = Database.queryWithBinds(
                'SELECT Id FROM Account LIMIT 1',
                new Map<String, Object>(),
                accessLevel
            );
            observation.put('result', 'ok');
            observation.put('rowCount', rows.size());
        } catch (Exception error) {
            observation.put('exceptionType', error.getTypeName());
            observation.put('exceptionMessage', error.getMessage());
        }
        System.assert(false, 'OBS|' + JSON.serialize(observation));
    }
}
