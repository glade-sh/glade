@IsTest private class ProfileCrudProbeTest {
    private static Map<String,Object> asProfile(String profileName) {
        List<Profile> ps = [SELECT Id FROM Profile WHERE Name = :profileName];
        if (ps.isEmpty()) return new Map<String,Object>{ 'profile' => 'absent' };
        String email = 'pcp' + Math.abs(Crypto.getRandomInteger()) + '@example.com';
        User u = new User(FirstName='T', LastName='U', Email=email, Username=email + UserInfo.getOrganizationId(),
            Alias='pcp', ProfileId=ps[0].Id, TimeZoneSidKey='America/Los_Angeles', LocaleSidKey='en_US',
            EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US');
        insert u;
        Map<String,Object> r;
        System.runAs(u) { r = ProfileCrudProbe.observeCurrentUser(); }
        return r;
    }
    @IsTest static void observe() {
        Map<String,Object> r = ProfileCrudProbe.observe();
        r.put('Standard User', asProfile('Standard User'));
        r.put('Standard Platform User', asProfile('Standard Platform User'));
        System.assert(false, 'OBS|' + JSON.serialize(r));
    }
}
