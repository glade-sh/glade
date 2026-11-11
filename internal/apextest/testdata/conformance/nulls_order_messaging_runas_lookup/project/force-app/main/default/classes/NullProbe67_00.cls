@IsTest(SeeAllData=false) private class NullProbe67_00 {
 @IsTest static void m00(){ System.assert(false,'CAPTURE|ConversationEntry|'+NullCore67.probeObject('ConversationEntry',new List<String>{'ActorId','ConversationEntityId'})); }
 @IsTest static void m01(){ System.assert(false,'CAPTURE|MessagingChannel|'+NullCore67.probeObject('MessagingChannel',new List<String>{'BusinessHoursId','FallbackQueueId','SessionHandlerId','TargetQueueId'})); }
 @IsTest static void m02(){ System.assert(false,'CAPTURE|MessagingEndUser|'+NullCore67.probeObject('MessagingEndUser',new List<String>{'AccountId','ContactId','LeadId'})); }
 @IsTest static void m03(){ System.assert(false,'CAPTURE|MessagingSession|'+NullCore67.probeObject('MessagingSession',new List<String>{'CampaignId','CaseId','ConversationId','LeadId','OpportunityId'})); }
 @IsTest static void m04(){ System.assert(false,'CAPTURE|MessagingSession|'+NullCore67.probeObject('MessagingSession',new List<String>{'TargetUserId'})); }
}
