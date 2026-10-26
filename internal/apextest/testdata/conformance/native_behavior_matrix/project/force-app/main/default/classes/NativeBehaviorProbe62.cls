@IsTest
private class NativeBehaviorProbe62 {
    private static User standardUser() {
        Profile profile = [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1];
        String suffix = String.valueOf(Crypto.getRandomInteger()).replace('-', 'n');
        return new User(
            Alias = 'matrix',
            Email = 'matrix@example.invalid',
            EmailEncodingKey = 'UTF-8',
            LastName = 'Matrix',
            LanguageLocaleKey = 'en_US',
            LocaleSidKey = 'en_US',
            TimeZoneSidKey = 'America/Los_Angeles',
            ProfileId = profile.Id,
            Username = 'matrix.' + UserInfo.getOrganizationId() + '.' + suffix + '@example.invalid'
        );
    }

    @IsTest static void standardProfileFieldAccess() {
        User actor = standardUser();
        insert actor;
        Boolean accessible;
        System.runAs(actor) {
            accessible = Contact.DoNotCall.getDescribe().isAccessible();
        }
        System.assert(false, 'PROFILE_FLS|' + accessible);
    }

    @IsTest static void genericTypeIdentity() {
        NativeBehaviorStubProvider provider = new NativeBehaviorStubProvider();
        NativeBehaviorTarget target = (NativeBehaviorTarget) Test.createStub(NativeBehaviorTarget.class, provider);
        target.accept(new List<Orchard__c>());
        System.assert(false, 'GENERIC_TYPE|' + provider.matchesLiteral + '|' + provider.parameterTypeName);
    }
}
