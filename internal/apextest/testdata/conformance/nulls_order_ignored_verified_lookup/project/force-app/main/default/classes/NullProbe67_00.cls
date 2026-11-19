@IsTest(SeeAllData=false) private class NullProbe67_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ContentDistribution|'+NullCore67.probeObject('ContentDistribution',new List<String>{'RelatedRecordId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|FeedItem|'+NullCore67.probeObject('FeedItem',new List<String>{'LastEditById','RelatedRecordId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|Order|'+NullCore67.probeObject('Order',new List<String>{'AccountId','BillToContactId','CompanyAuthorizedById','CustomerAuthorizedById','OriginalOrderId'})); }
}
