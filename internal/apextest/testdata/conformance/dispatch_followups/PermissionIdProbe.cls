public class PermissionIdProbe {
    public static String observe(String mode) {
        Id fakeId = Id.valueOf(PermissionSet.SObjectType.getDescribe().getKeyPrefix() + '000000000999');
        try {
            AccessLevel scoped;
            if (mode == 'string') scoped = AccessLevel.USER_MODE.withPermissionSetId(String.valueOf(fakeId));
            else scoped = AccessLevel.USER_MODE.withPermissionSetId(fakeId);
            return 'ok|' + String.valueOf(scoped);
        } catch (Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    }
}
