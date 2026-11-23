@IsTest
private class R5RtProbe {
  @IsTest static void avail() {
    Map<String, Schema.RecordTypeInfo> m = Schema.SObjectType.Rt__c.getRecordTypeInfosByDeveloperName();
    List<String> parts = new List<String>();
    for (String k : new List<String>{'A','B','C'}) {
      Schema.RecordTypeInfo i = m.get(k);
      parts.add(k + '=' + (i == null ? 'none' : String.valueOf(i.isAvailable())));
    }
    Boolean ins;
    try { insert new Rt__c(Name = 'x', RecordTypeId = m.get('B').getRecordTypeId()); ins = true; } catch (Exception e) { ins = false; parts.add(e.getMessage()); }
    parts.add('insertB=' + ins);
    System.assert(false, 'R5|rt|' + String.join(parts, ','));
  }
}
