public class CaptureMixedDml {
    public static User actor(Boolean withRole) {
        User u = new User(Alias='capmix', Email='mixed@example.invalid', EmailEncodingKey='UTF-8',
            LastName='CaptureMixed', LanguageLocaleKey='en_US', LocaleSidKey='en_US',
            TimeZoneSidKey='America/Los_Angeles', ProfileId=UserInfo.getProfileId(),
            Username=String.valueOf(Datetime.now().getTime())+'.'+UserInfo.getOrganizationId()+'@mixed.example.invalid');
        if (withRole) u.UserRoleId=[SELECT Id FROM UserRole WHERE DeveloperName='CaptureRole' LIMIT 1].Id;
        return u;
    }
    public static String observe(Boolean withRole) {
        try { insert new Account(Name='Capture Mixed'); insert actor(withRole); return 'success'; }
        catch (Exception e) { return e.getTypeName()+': '+e.getMessage(); }
    }
}
