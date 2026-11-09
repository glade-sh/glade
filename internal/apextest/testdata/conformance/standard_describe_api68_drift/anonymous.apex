for (String objectName : new List<String>{'AssetStatePeriod', 'Conversation', 'CronJobDetail'}) {
  Schema.SObjectType token = Schema.getGlobalDescribe().get(objectName);
  Map<String,Schema.SObjectField> fields = token.getDescribe().fields.getMap();
  System.debug(LoggingLevel.ERROR, 'API68CHECK|' + objectName + '|ApplUnitPriceUpliftPct=' + fields.containsKey('ApplUnitPriceUpliftPct') + '|RampUpliftType=' + fields.containsKey('RampUpliftType') + '|ActiveModalities=' + fields.containsKey('ActiveModalities') + '|CronJobType=' + fields.containsKey('CronJobType'));
}
