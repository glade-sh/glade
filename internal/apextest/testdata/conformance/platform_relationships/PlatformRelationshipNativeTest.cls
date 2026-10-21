@IsTest public class PlatformRelationshipNativeTest {
    @IsTest static void sweepAccount() { PlatformRelationshipProbe.sweep('Account'); System.assert(false,'PLATFORM_ROWS|'+PlatformRelationshipProbe.compact()); }
    @IsTest static void sweepUser() { PlatformRelationshipProbe.sweep('User'); System.assert(false,'PLATFORM_ROWS|'+PlatformRelationshipProbe.compact()); }
    @IsTest static void sweepCase() { PlatformRelationshipProbe.sweep('Case'); System.assert(false,'PLATFORM_ROWS|'+PlatformRelationshipProbe.compact()); }
    @IsTest static void sweepContact() { PlatformRelationshipProbe.sweep('Contact'); System.assert(false,'PLATFORM_ROWS|'+PlatformRelationshipProbe.compact()); }
    @IsTest static void sweepOpportunity() { PlatformRelationshipProbe.sweep('Opportunity'); System.assert(false,'PLATFORM_ROWS|'+PlatformRelationshipProbe.compact()); }
    @IsTest static void sweepLinkProbe() { PlatformRelationshipProbe.sweep('LinkProbe__c'); System.assert(false,'PLATFORM_ROWS|'+PlatformRelationshipProbe.compact()); }
    @IsTest static void accessRows() { PlatformRelationshipProbe.accessRows(); System.assert(false,'PLATFORM_ROWS|'+PlatformRelationshipProbe.compact()); }
    @IsTest static void staticSlice() {
        List<User> users=[SELECT Id,(SELECT Id FROM Badges),(SELECT Id FROM CombinedAttachments),(SELECT Id FROM CommSubscriptionConsents),(SELECT Id FROM ContactRequests) FROM User LIMIT 1];
        Account a=new Account(Name='Static probe'); insert a;
        Contact c=new Contact(LastName='Static probe',AccountId=a.Id); insert c;
        LinkProbe__c link=new LinkProbe__c(Name='Static probe',Account__c=a.Id); insert link;
        Account row=[SELECT Id,UserRecordAccess.HasReadAccess,(SELECT Id FROM ProbeItems__r),(SELECT Id,UserRecordAccess.HasEditAccess FROM Contacts),(SELECT Id FROM Attachments) FROM Account WHERE Id=:a.Id];
        System.assertEquals(true,row.UserRecordAccess.HasReadAccess);
        System.assertEquals(1,row.ProbeItems__r.size());
        System.assertEquals(true,row.Contacts[0].UserRecordAccess.HasEditAccess);
        System.debug(LoggingLevel.ERROR,'PLATFORM_STATIC|ok');
    }
}
