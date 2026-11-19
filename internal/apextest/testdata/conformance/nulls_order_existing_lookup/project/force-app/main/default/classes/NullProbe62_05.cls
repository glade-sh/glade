@IsTest(SeeAllData=true) private class NullProbe62_05 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|RecordType|'+NullExistingCore62.probeObject('RecordType',new List<String>{'BusinessProcessId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|Refund|'+NullExistingCore62.probeObject('Refund',new List<String>{'AccountId','PaymentGatewayId','PaymentGroupId','PaymentMethodId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|RefundLinePayment|'+NullExistingCore62.probeObject('RefundLinePayment',new List<String>{'AssociatedAccountId','AssociatedRefundLinePaymentId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|ReturnOrder|'+NullExistingCore62.probeObject('ReturnOrder',new List<String>{'OrderId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|ReturnOrderLineItem|'+NullExistingCore62.probeObject('ReturnOrderLineItem',new List<String>{'AssetId','DestinationLocationId','OrderItemId','Product2Id','ProductItemId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|ReturnOrderLineItem|'+NullExistingCore62.probeObject('ReturnOrderLineItem',new List<String>{'ProductRequestLineItemId','ProductServiceCampaignItemId','SourceLocationId'})); }
 @IsTest static void m06(){ System.assert(false,'CAPTURE|SPSamlAttributes|'+NullExistingCore62.probeObject('SPSamlAttributes',new List<String>{'ConnectivityId'})); }
 @IsTest static void m07(){ System.assert(false,'CAPTURE|ScorecardMetric|'+NullExistingCore62.probeObject('ScorecardMetric',new List<String>{'ReportId'})); }
 @IsTest static void m08(){ System.assert(false,'CAPTURE|Seller|'+NullExistingCore62.probeObject('Seller',new List<String>{'PartyId'})); }
 @IsTest static void m09(){ System.assert(false,'CAPTURE|SerializedProduct|'+NullExistingCore62.probeObject('SerializedProduct',new List<String>{'AssetId','ProductItemId'})); }
 @IsTest static void m10(){ System.assert(false,'CAPTURE|ServiceAppointment|'+NullExistingCore62.probeObject('ServiceAppointment',new List<String>{'BundlePolicyId','ParentRecordId','RelatedBundleId','ServiceTerritoryId'})); }
 @IsTest static void m11(){ System.assert(false,'CAPTURE|ServiceResource|'+NullExistingCore62.probeObject('ServiceResource',new List<String>{'AccountId','LocationId','PayGroupId','RelatedRecordId','SchedulingConstraintId'})); }
 @IsTest static void m12(){ System.assert(false,'CAPTURE|ServiceResource|'+NullExistingCore62.probeObject('ServiceResource',new List<String>{'ServiceCrewId'})); }
 @IsTest static void m13(){ System.assert(false,'CAPTURE|ServiceTerritoryMember|'+NullExistingCore62.probeObject('ServiceTerritoryMember',new List<String>{'OperatingHoursId','TravelModeId'})); }
 @IsTest static void m14(){ System.assert(false,'CAPTURE|Shift|'+NullExistingCore62.probeObject('Shift',new List<String>{'JobProfileId','RecordsetFilterCriteriaId','ServiceResourceId','ServiceTerritoryId','ShiftTemplateId'})); }
 @IsTest static void m15(){ System.assert(false,'CAPTURE|Skill|'+NullExistingCore62.probeObject('Skill',new List<String>{'TypeId'})); }
 @IsTest static void m16(){ System.assert(false,'CAPTURE|Swarm|'+NullExistingCore62.probeObject('Swarm',new List<String>{'CollaborationRoomId'})); }
 @IsTest static void m17(){ System.assert(false,'CAPTURE|ThreatDetectionFeedback|'+NullExistingCore62.probeObject('ThreatDetectionFeedback',new List<String>{'ThreatDetectionEventId','UserId'})); }
 @IsTest static void m18(){ System.assert(false,'CAPTURE|TimeSheet|'+NullExistingCore62.probeObject('TimeSheet',new List<String>{'ServiceResourceId'})); }
 @IsTest static void m19(){ System.assert(false,'CAPTURE|TimeSheetEntry|'+NullExistingCore62.probeObject('TimeSheetEntry',new List<String>{'WorkOrderId','WorkOrderLineItemId'})); }
}
