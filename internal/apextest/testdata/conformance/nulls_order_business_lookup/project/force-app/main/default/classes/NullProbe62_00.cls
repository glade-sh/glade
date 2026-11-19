@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|AccountPartner|'+NullCore62.probeObject('AccountPartner',new List<String>{'OpportunityId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|CampaignMember|'+NullCore62.probeObject('CampaignMember',new List<String>{'ContactId','LeadId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|Order|'+NullCore62.probeObject('Order',new List<String>{'AccountId','BillToContactId','CompanyAuthorizedById','CustomerAuthorizedById','OriginalOrderId'})); }
}
