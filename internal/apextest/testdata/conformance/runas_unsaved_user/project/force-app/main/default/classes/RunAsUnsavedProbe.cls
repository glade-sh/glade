public class RunAsUnsavedProbe {
    static User build(String tag, Id profileId) {
        return new User(LastName = 'Tester' + tag, FirstName = 'beta', Alias = 'rau' + tag, Email = 'rau' + tag + '@example.invalid',
            Username = 'rau' + tag + '.' + String.valueOf(System.currentTimeMillis()) + '@example.invalid', ProfileId = profileId,
            TimeZoneSidKey = 'America/Los_Angeles', LocaleSidKey = 'en_US', EmailEncodingKey = 'UTF-8', LanguageLocaleKey = 'en_US', IsActive = true);
    }
    static Map<String, Object> inside(User u, Id origId, String origName) {
        Map<String, Object> o = new Map<String, Object>();
        o.put('sameIdAsRunner', UserInfo.getUserId() == origId);
        o.put('idNull', UserInfo.getUserId() == null);
        o.put('nameIsRunnerName', UserInfo.getName() == origName);
        o.put('firstNameIsBeta', UserInfo.getFirstName() == 'beta');
        o.put('lastNameFromUser', UserInfo.getLastName() == u.LastName);
        o.put('userNameFromUser', UserInfo.getUserName() == u.Username);
        o.put('profileFromUser', UserInfo.getProfileId() == u.ProfileId);
        List<User> q = [SELECT Id, Name FROM User WHERE Id = :UserInfo.getUserId()];
        o.put('queryByUserInfoIdSize', q.size());
        o.put('queryNameNull', q.isEmpty() ? null : (Object) (q[0].Name == null));
        o.put('queryNameEqUserInfoName', q.isEmpty() ? null : (Object) (q[0].Name == UserInfo.getName()));
        return o;
    }
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        Id origId = UserInfo.getUserId();
        String origName = UserInfo.getName();
        Id pid = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1].Id;
        User unsaved = build('a', pid);
        System.runAs(unsaved) { o.put('unsavedInside', inside(unsaved, origId, origName)); }
        o.put('unsavedIdAfterNull', unsaved.Id == null);
        o.put('unsavedUserRowsAfter', [SELECT COUNT() FROM User WHERE Username = :unsaved.Username]);
        User saved = build('b', pid);
        insert saved;
        System.runAs(saved) { o.put('savedInside', inside(saved, origId, origName)); }
        o.put('savedIdSetAfter', saved.Id != null);
        return o;
    }
}
