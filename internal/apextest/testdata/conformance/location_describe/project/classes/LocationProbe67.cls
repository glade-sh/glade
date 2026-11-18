@IsTest private class LocationProbe67 {
 @IsTest static void capture() {
  Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
  System.assertEquals(true, fields.containsKey('Probe_Point__c'));
  System.assertEquals(true, fields.containsKey('Probe_Point__Latitude__s'));
  System.assertEquals(true, fields.containsKey('probe_point__longitude__s'));
  System.assertEquals('Probe_Point__c', fields.get('Probe_Point__Latitude__s').getDescribe().getCompoundFieldName());
  System.assertEquals('Probe_Point__c', fields.get('probe_point__longitude__s').getDescribe().getCompoundFieldName());
  System.assertEquals(true, fields.containsKey('Probe_Text__c'));
  System.assertEquals(false, fields.containsKey('Probe_Text__Latitude__s'));
  System.assertEquals(false, fields.containsKey('Absent__Latitude__s'));
  for (String name : new List<String>{'Probe_Point__c','Probe_Point__Latitude__s','probe_point__longitude__s','Probe_Text__c','Probe_Text__Latitude__s','Absent__Latitude__s'}) {
   Schema.SObjectField f = fields.get(name);
   System.debug('CAPTURE|' + name + '|' + fields.containsKey(name) + '|' + (f == null ? 'null' : f.getDescribe().getName()) + '|' + (f == null ? 'null' : String.valueOf(f.getDescribe().getCompoundFieldName())));
  }
 }
}
