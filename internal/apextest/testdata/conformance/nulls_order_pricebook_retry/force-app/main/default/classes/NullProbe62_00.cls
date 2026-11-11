@IsTest(SeeAllData=false) private class NullProbe62_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|OpportunityLineItem|'+NullCore62.probeObject('OpportunityLineItem',new List<String>{'PricebookEntryId','Product2Id'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|OrderItem|'+NullCore62.probeObject('OrderItem',new List<String>{'Product2Id'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|ProductConsumed|'+NullCore62.probeObject('ProductConsumed',new List<String>{'PricebookEntryId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|WorkOrderLineItem|'+NullCore62.probeObject('WorkOrderLineItem',new List<String>{'Product2Id'})); }
}
