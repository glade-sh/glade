@IsTest(SeeAllData=false) private class NullProbe67_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|OpportunityLineItem|'+NullCore67.probeObject('OpportunityLineItem',new List<String>{'PricebookEntryId','Product2Id'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|OrderItem|'+NullCore67.probeObject('OrderItem',new List<String>{'Product2Id'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|ProductConsumed|'+NullCore67.probeObject('ProductConsumed',new List<String>{'PricebookEntryId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore67.probeObject('WorkOrderLineItem',new List<String>{'Product2Id'})); }
}
