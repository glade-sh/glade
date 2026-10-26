@IsTest private with sharing class CaptureSharingWithTest {
    @TestSetup static void seed() {
        Profile p = [SELECT Id FROM Profile WHERE Name = 'Minimum Access - Salesforce' LIMIT 1];
        List<User> actors = new List<User>();
        for (String mode : new List<String>{'Read','ViewAll','None'}) {
            actors.add(new User(Alias = 'cap' + mode.left(4), Email = 'capture@example.invalid',
                EmailEncodingKey = 'UTF-8', LastName = 'Capture' + mode, LanguageLocaleKey = 'en_US',
                LocaleSidKey = 'en_US', TimeZoneSidKey = 'America/Los_Angeles', ProfileId = p.Id,
                Username = mode + '.' + UserInfo.getOrganizationId() + '@capture.example.invalid'));
        }
        insert actors;
        List<PermissionSetAssignment> assignments = new List<PermissionSetAssignment>();
        for (PermissionSet ps : [SELECT Id, Name FROM PermissionSet WHERE Name IN ('CaptureRead','CaptureViewAll')]) {
            for (User actor : actors) {
                if ('Capture' + actor.LastName.substring(7) == ps.Name) assignments.add(new PermissionSetAssignment(AssigneeId=actor.Id, PermissionSetId=ps.Id));
            }
        }
        insert assignments;
        System.runAs(new User(Id=UserInfo.getUserId())) { insert new CapturePrivate__c(Name='Admin owned'); }
    }
    @IsTest static void observeRead() {
        User actor = [SELECT Id FROM User WHERE LastName='CaptureRead' LIMIT 1];
        Map<String,Object> result = new Map<String,Object>();
        System.runAs(actor) {
            try { result.put('default', [SELECT Id FROM CapturePrivate__c].size()); } catch (Exception e) { result.put('default', e.getTypeName() + ': ' + e.getMessage()); }
            try { result.put('system', [SELECT Id FROM CapturePrivate__c WITH SYSTEM_MODE].size()); } catch (Exception e) { result.put('system', e.getTypeName() + ': ' + e.getMessage()); }
            try { result.put('user', [SELECT Id FROM CapturePrivate__c WITH USER_MODE].size()); } catch (Exception e) { result.put('user', e.getTypeName() + ': ' + e.getMessage()); }
            result.put('helper_omitted',CaptureSharingHelperOmitted.observe());
            result.put('helper_without',CaptureSharingHelperWithout.observe());
            result.put('helper_with',CaptureSharingHelperWith.observe());
        }
        System.assert(false, 'OBS|'+JSON.serialize(result));
    }
    @IsTest static void observeViewAll() {
        User actor = [SELECT Id FROM User WHERE LastName='CaptureViewAll' LIMIT 1];
        Map<String,Object> result = new Map<String,Object>();
        System.runAs(actor) {
            try { result.put('default', [SELECT Id FROM CapturePrivate__c].size()); } catch (Exception e) { result.put('default', e.getTypeName() + ': ' + e.getMessage()); }
            try { result.put('system', [SELECT Id FROM CapturePrivate__c WITH SYSTEM_MODE].size()); } catch (Exception e) { result.put('system', e.getTypeName() + ': ' + e.getMessage()); }
            try { result.put('user', [SELECT Id FROM CapturePrivate__c WITH USER_MODE].size()); } catch (Exception e) { result.put('user', e.getTypeName() + ': ' + e.getMessage()); }
            result.put('helper_omitted',CaptureSharingHelperOmitted.observe());
            result.put('helper_without',CaptureSharingHelperWithout.observe());
            result.put('helper_with',CaptureSharingHelperWith.observe());
        }
        System.assert(false, 'OBS|'+JSON.serialize(result));
    }
    @IsTest static void observeNone() {
        User actor = [SELECT Id FROM User WHERE LastName='CaptureNone' LIMIT 1];
        Map<String,Object> result = new Map<String,Object>();
        System.runAs(actor) {
            try { result.put('default', [SELECT Id FROM CapturePrivate__c].size()); } catch (Exception e) { result.put('default', e.getTypeName() + ': ' + e.getMessage()); }
            try { result.put('system', [SELECT Id FROM CapturePrivate__c WITH SYSTEM_MODE].size()); } catch (Exception e) { result.put('system', e.getTypeName() + ': ' + e.getMessage()); }
            try { result.put('user', [SELECT Id FROM CapturePrivate__c WITH USER_MODE].size()); } catch (Exception e) { result.put('user', e.getTypeName() + ': ' + e.getMessage()); }
            result.put('helper_omitted',CaptureSharingHelperOmitted.observe());
            result.put('helper_without',CaptureSharingHelperWithout.observe());
            result.put('helper_with',CaptureSharingHelperWith.observe());
        }
        System.assert(false, 'OBS|'+JSON.serialize(result));
    }
}
