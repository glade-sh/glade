@IsTest(SeeAllData=false) private class NullProbe62_05 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|MessagingEndUser|'+NullCore62.probeObject('MessagingEndUser',new List<String>{'AccountId','ContactId','LeadId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|MessagingSession|'+NullCore62.probeObject('MessagingSession',new List<String>{'CaseId','LeadId','OpportunityId','CampaignId','TargetUserId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|MessagingSession|'+NullCore62.probeObject('MessagingSession',new List<String>{'ConversationId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|MobileSettingsAssignment|'+NullCore62.probeObject('MobileSettingsAssignment',new List<String>{'ProfileId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|MsgChannelLanguageKeyword|'+NullCore62.probeObject('MsgChannelLanguageKeyword',new List<String>{'MessagingChannelUsageId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|MyDomainDiscoverableLogin|'+NullCore62.probeObject('MyDomainDiscoverableLogin',new List<String>{'ExecuteApexHandlerAsId'})); }
 @IsTest static void m06(){ System.assert(false,'CAPTURE|OauthTokenExchHandlerApp|'+NullCore62.probeObject('OauthTokenExchHandlerApp',new List<String>{'ExternalClientApplicationId','ConnectedApplicationId'})); }
 @IsTest static void m07(){ System.assert(false,'CAPTURE|Opportunity|'+NullCore62.probeObject('Opportunity',new List<String>{'AccountId','CampaignId','Pricebook2Id','ContactId'})); }
 @IsTest static void m08(){ System.assert(false,'CAPTURE|OpportunityLineItem|'+NullCore62.probeObject('OpportunityLineItem',new List<String>{'PricebookEntryId','Product2Id'})); }
 @IsTest static void m09(){ System.assert(false,'CAPTURE|Order|'+NullCore62.probeObject('Order',new List<String>{'ContractId','AccountId','Pricebook2Id','OriginalOrderId','CustomerAuthorizedById'})); }
 @IsTest static void m10(){ System.assert(false,'CAPTURE|Order|'+NullCore62.probeObject('Order',new List<String>{'CompanyAuthorizedById','BillToContactId','ShipToContactId'})); }
 @IsTest static void m11(){ System.assert(false,'CAPTURE|OrderItem|'+NullCore62.probeObject('OrderItem',new List<String>{'Product2Id','OriginalOrderItemId'})); }
 @IsTest static void m12(){ System.assert(false,'CAPTURE|OrgMetric|'+NullCore62.probeObject('OrgMetric',new List<String>{'LatestOrgMetricScanSummaryId'})); }
 @IsTest static void m13(){ System.assert(false,'CAPTURE|OutboundEngmtContentRsrc|'+NullCore62.probeObject('OutboundEngmtContentRsrc',new List<String>{'ResourceReferenceRecordId','TargetReferenceRecordId'})); }
 @IsTest static void m14(){ System.assert(false,'CAPTURE|OutgoingEmailRelation|'+NullCore62.probeObject('OutgoingEmailRelation',new List<String>{'RelationId'})); }
 @IsTest static void m15(){ System.assert(false,'CAPTURE|Partner|'+NullCore62.probeObject('Partner',new List<String>{'OpportunityId','AccountFromId'})); }
 @IsTest static void m16(){ System.assert(false,'CAPTURE|PartyConsent|'+NullCore62.probeObject('PartyConsent',new List<String>{'PartyRoleId','BusinessBrandId','DataUsePurposeId'})); }
 @IsTest static void m17(){ System.assert(false,'CAPTURE|Payment|'+NullCore62.probeObject('Payment',new List<String>{'PaymentGroupId','AccountId','PaymentAuthorizationId','PaymentGatewayId','PaymentMethodId'})); }
 @IsTest static void m18(){ System.assert(false,'CAPTURE|PaymentAuthAdjustment|'+NullCore62.probeObject('PaymentAuthAdjustment',new List<String>{'AccountId'})); }
 @IsTest static void m19(){ System.assert(false,'CAPTURE|PaymentAuthorization|'+NullCore62.probeObject('PaymentAuthorization',new List<String>{'PaymentGroupId','AccountId','PaymentMethodId','PaymentGatewayId'})); }
}
