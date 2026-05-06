public class SecurityProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'security.user-info') {
            return runUserInfo();
        } else if (probeId == 'security.profile-name') {
            return runProfileName();
        } else if (probeId == 'security.crud-check') {
            return runCrudCheck();
        } else if (probeId == 'security.fls-check') {
            return runFlsCheck();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runUserInfo() {
        return UserInfo.getUserName() != null;
    }

    private Object runProfileName() {
        return UserInfo.getProfileId() != null;
    }

    private Object runCrudCheck() {
        return Account.SObjectType.getDescribe().isAccessible();
    }

    private Object runFlsCheck() {
        Map<String, Schema.SObjectField> fieldMap = Account.SObjectType.getDescribe().fields.getMap();
        Schema.SObjectField nameField = fieldMap.get('Name');
        return nameField.getDescribe().isAccessible();
    }
}
