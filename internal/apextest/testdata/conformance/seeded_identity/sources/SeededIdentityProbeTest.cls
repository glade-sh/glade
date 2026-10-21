@IsTest
private class SeededIdentityProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'PROBE|' + JSON.serialize(SeededIdentityProbe.observe()));
    }
    @IsTest static void fabricatedProfileRunAs() {
        String outcome = 'entered';
        User actor = new User(
            Username = 'identity.' + DateTime.now().getTime() + '@example.invalid',
            Email = 'identity@example.invalid', Alias = 'identity', LastName = 'Identity',
            EmailEncodingKey = 'UTF-8', LanguageLocaleKey = 'en_US', LocaleSidKey = 'en_US',
            TimeZoneSidKey = 'America/Los_Angeles',
            ProfileId = SeededIdentityProbe.fakeId(Profile.SObjectType, 1)
        );
        try {
            System.runAs(actor) {}
        } catch (Exception ex) {
            outcome = ex.getTypeName() + ':' + ex.getMessage();
        }
        System.assert(false, 'PROFILE|' + outcome);
    }
    @IsTest static void fabricatedUserRunAs() {
        System.runAs(new User(Id = SeededIdentityProbe.fakeId(User.SObjectType, 1))) {
            System.assert(false, 'PROBE|entered');
        }
    }
}
