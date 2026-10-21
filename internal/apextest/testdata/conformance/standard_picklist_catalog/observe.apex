Map<String,Schema.SObjectType> types = Schema.getGlobalDescribe();
List<String> keys = new List<String>{'AllergyIntoleranceHistory.Field','CareProgramHistory.Field','ConversationChannelDefinition.CustomEventPayloadField','ConversationChannelDefinition.CustomEventTypeField','CustodyChainEntry.ReferenceObjectApiName','CustodyItem.ReferenceItemObjectApiName','DigitalVerificationSetup.ReferenceObjectApiName','ESignatureEnvelopeConfig.TargetObjectName','HealthConditionHistory.Field','MedicationRequestHistory.Field','MedicationStatementHistory.Field','MemberPlanHistory.Field','PatientImmunizationHistory.Field','PatientMedicalProcedureHistory.Field','RecAlrtDataSrcExpSetDef.ExpressionSetObject','Account.Industry'};
for (String key : keys) {
 List<String> parts = key.split('\\.');
 Schema.SObjectType token = types.get(parts[0]);
 if (token == null) { System.debug('CATALOG|' + key + '|UNAVAILABLE'); continue; }
 Schema.SObjectField field = token.getDescribe().fields.getMap().get(parts[1]);
 if (field == null) { System.debug('CATALOG|' + key + '|UNAVAILABLE'); continue; }
 Integer customCount = 0;
 Integer standardCount = 0;
 for (Schema.PicklistEntry entry : field.getDescribe().getPicklistValues()) {
  String value = entry.getValue();
  if (value.endsWith('__c') || value.endsWith('__e') || value.endsWith('__mdt') || value.endsWith('__x')) { customCount++; } else { standardCount++; }
 }
 System.debug('CATALOG|' + key + '|' + customCount + '|' + standardCount);
}
