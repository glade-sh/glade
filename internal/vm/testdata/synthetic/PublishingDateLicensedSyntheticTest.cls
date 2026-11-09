@IsTest private class PublishingDateLicensedSyntheticTest {
    @TestSetup static void licenseActor() {
        User actor = [SELECT Id, UserPermissionsKnowledgeUser FROM User WHERE Id = :UserInfo.getUserId()];
        actor.UserPermissionsKnowledgeUser = true;
        update actor;
    }
    @IsTest static void observe() { System.assert(false, 'OBS|'+JSON.serialize(PublishingDateArgument.observe())); }
}
