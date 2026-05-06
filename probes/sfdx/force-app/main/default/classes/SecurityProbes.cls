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
        return Schema.sObjectType.Account.isAccessible();
    }

    private Object runFlsCheck() {
        return Schema.sObjectType.Account.fields.Name.isAccessible();
    }
}
