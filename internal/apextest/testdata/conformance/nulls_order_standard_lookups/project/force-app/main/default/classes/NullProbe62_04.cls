@IsTest(SeeAllData=false) private class NullProbe62_04 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|FeedComment|'+NullCore62.probeObject('FeedComment',new List<String>{'LastEditById','RelatedRecordId','ThreadParentId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|FeedItem|'+NullCore62.probeObject('FeedItem',new List<String>{'LastEditById','RelatedRecordId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|FinanceTransaction|'+NullCore62.probeObject('FinanceTransaction',new List<String>{'ReferenceEntityId','AccountId','SourceEntityId','DestinationEntityId','LegalEntityId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|FinanceTransaction|'+NullCore62.probeObject('FinanceTransaction',new List<String>{'ParentReferenceEntityId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|FlowOrchestrationInstance|'+NullCore62.probeObject('FlowOrchestrationInstance',new List<String>{'InterviewId'})); }
 @IsTest static void m05(){ System.assert(false,'CAPTURE|FlowOrchestrationWorkItem|'+NullCore62.probeObject('FlowOrchestrationWorkItem',new List<String>{'StepInstanceId','RelatedRecordId','ParentWorkItemId'})); }
 @IsTest static void m06(){ System.assert(false,'CAPTURE|Folder|'+NullCore62.probeObject('Folder',new List<String>{'ParentId'})); }
 @IsTest static void m07(){ System.assert(false,'CAPTURE|GtwyProvPaymentMethodType|'+NullCore62.probeObject('GtwyProvPaymentMethodType',new List<String>{'PaymentGatewayProviderId','RecordTypeId'})); }
 @IsTest static void m08(){ System.assert(false,'CAPTURE|Image|'+NullCore62.probeObject('Image',new List<String>{'ContentDocumentId'})); }
 @IsTest static void m09(){ System.assert(false,'CAPTURE|Lead|'+NullCore62.probeObject('Lead',new List<String>{'DandbCompanyId','IndividualId'})); }
 @IsTest static void m10(){ System.assert(false,'CAPTURE|LightningExperienceTheme|'+NullCore62.probeObject('LightningExperienceTheme',new List<String>{'DefaultBrandingSetId'})); }
 @IsTest static void m11(){ System.assert(false,'CAPTURE|LightningOnboardingConfig|'+NullCore62.probeObject('LightningOnboardingConfig',new List<String>{'CollaborationGroupId'})); }
 @IsTest static void m12(){ System.assert(false,'CAPTURE|ListEmail|'+NullCore62.probeObject('ListEmail',new List<String>{'CampaignId','RelatedToId'})); }
 @IsTest static void m13(){ System.assert(false,'CAPTURE|ListEmailSentResult|'+NullCore62.probeObject('ListEmailSentResult',new List<String>{'RecipientId','ActivityId'})); }
 @IsTest static void m14(){ System.assert(false,'CAPTURE|LiveChatTranscriptEvent|'+NullCore62.probeObject('LiveChatTranscriptEvent',new List<String>{'AgentId'})); }
 @IsTest static void m15(){ System.assert(false,'CAPTURE|Location|'+NullCore62.probeObject('Location',new List<String>{'ParentLocationId','VisitorAddressId','LogoId','AccountId'})); }
 @IsTest static void m16(){ System.assert(false,'CAPTURE|MaintenanceAsset|'+NullCore62.probeObject('MaintenanceAsset',new List<String>{'WorkTypeId','ContractLineItemId'})); }
 @IsTest static void m17(){ System.assert(false,'CAPTURE|MaintenancePlan|'+NullCore62.probeObject('MaintenancePlan',new List<String>{'WorkTypeId','AccountId','ServiceContractId','LocationId','ContactId'})); }
 @IsTest static void m18(){ System.assert(false,'CAPTURE|MaintenanceWorkRule|'+NullCore62.probeObject('MaintenanceWorkRule',new List<String>{'ParentMaintenanceRecordId','WorkTypeId','ParentMaintenancePlanId','RecordsetFilterCriteriaId'})); }
 @IsTest static void m19(){ System.assert(false,'CAPTURE|MessagingChannel|'+NullCore62.probeObject('MessagingChannel',new List<String>{'TargetQueueId','SessionHandlerId','FallbackQueueId','BusinessHoursId'})); }
}
