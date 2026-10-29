@IsTest
private class FFAssignmentProbeTest {
    static User newUser(Id profileId, String key) {
        return new User(Alias=key, Email='synthetic@example.invalid', EmailEncodingKey='UTF-8', LastName='Synthetic', LanguageLocaleKey='en_US', LocaleSidKey='en_US', ProfileId=profileId, TimeZoneSidKey='America/Los_Angeles', Username='flowfollowups.' + key + '.' + String.valueOf(DateTime.now().getTime()) + '@example.invalid');
    }
    static Map<String, Object> insertLead(String company, Id owner, Id original) {
        Lead lead = new Lead(LastName='Synthetic', Company=company, OwnerId=owner);
        Database.DMLOptions options = new Database.DMLOptions();
        options.assignmentRuleHeader.useDefaultRule = true;
        Database.SaveResult result = Database.insert(lead, options);
        if (!result.isSuccess()) return new Map<String, Object>{'status'=>String.valueOf(result.getErrors()[0].getStatusCode())};
        lead = [SELECT OwnerId FROM Lead WHERE Id=:lead.Id];
        String queueName;
        List<Group> queues = [SELECT DeveloperName FROM Group WHERE Id=:lead.OwnerId AND Type='Queue'];
        if (!queues.isEmpty()) queueName = queues[0].DeveloperName;
        return new Map<String, Object>{'status'=>'SUCCESS', 'queue'=>queueName};
    }
    @IsTest static void observe() {
        User original = [SELECT Id, ProfileId FROM User WHERE Id=:UserInfo.getUserId()];
        Profile standard = [SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
        User secondAdmin = newUser(original.ProfileId, 'ffadm');
        User standardUser = newUser(standard.Id, 'ffstd');
        insert new List<User>{secondAdmin, standardUser};
        Map<String, Object> out = new Map<String, Object>();
        System.runAs(original) {
            out.put('AR_original_matching_owner', insertLead('FF_USER', original.Id, original.Id));
            out.put('AR_original_nonmatching_owner', insertLead('FF_USER', secondAdmin.Id, original.Id));
            out.put('AR_original_profile_admin', insertLead('FF_PROFILE', original.Id, original.Id));
            System.runAs(secondAdmin) {
                out.put('AR_runAs_matching_owner', insertLead('FF_USER', secondAdmin.Id, original.Id));
                out.put('AR_runAs_nonmatching_owner', insertLead('FF_USER', original.Id, original.Id));
                out.put('AR_runAs_nonmatching_company', insertLead('FF_OTHER', secondAdmin.Id, original.Id));
            }
            System.runAs(standardUser) {
                out.put('AR_runAs_profile_standard', insertLead('FF_PROFILE', standardUser.Id, original.Id));
                out.put('AR_runAs_profile_rejection', insertLead('FF_OTHER', standardUser.Id, original.Id));
            }
            out.put('AR_restored_matching_owner', insertLead('FF_USER', original.Id, original.Id));
            out.put('AR_restored_profile_admin', insertLead('FF_PROFILE', original.Id, original.Id));
        }
        System.assert(false, 'FF_OTHER|' + JSON.serialize(out));
    }
}
