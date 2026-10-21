public class SeededIdentityProbe {
    public static Id fakeId(Schema.SObjectType kind, Integer counter) {
        String suffix = String.valueOf(counter);
        while (suffix.length() < 12) suffix = '0' + suffix;
        return (Id)(kind.getDescribe().getKeyPrefix() + suffix);
    }
    public static Map<String, String> observe() {
        Id fakeUser = fakeId(User.SObjectType, 1);
        Id fakeProfile = fakeId(Profile.SObjectType, 1);
        return new Map<String, String>{
            'I001' => String.valueOf(fakeUser != UserInfo.getUserId()),
            'I002' => String.valueOf(fakeProfile != UserInfo.getProfileId()),
            'I003' => String.valueOf([SELECT count() FROM User WHERE Id = :fakeUser]),
            'I004' => String.valueOf([SELECT count() FROM Profile WHERE Id = :fakeProfile]),
            'I005' => String.valueOf(fakeUser.getSObjectType()),
            'I006' => String.valueOf(fakeProfile.getSObjectType())
        };
    }
}
