Map<String,List<String>> probes=new Map<String,List<String>>{
 'Product2'=>new List<String>{'IsActive','IsSerialized'},
 'Contact'=>new List<String>{'HasOptedOutOfEmail','DoNotCall'},
 'Lead'=>new List<String>{'HasOptedOutOfFax','DoNotCall'},
 'Opportunity'=>new List<String>{'IsPrivate'},
 'Task'=>new List<String>{'IsReminderSet'}
};
for(String obj:probes.keySet()) {
 Schema.SObjectType typ=Schema.getGlobalDescribe().get(obj);
 SObject blank=typ.newSObject(null,false); SObject loaded=typ.newSObject(null,true);
 for(String field:probes.get(obj)) {
  try {
  Boolean before=blank.getPopulatedFieldsAsMap().containsKey(field);
  Object value=blank.get(field);
  Boolean after=blank.getPopulatedFieldsAsMap().containsKey(field);
  blank.put(field,false);
  Boolean written=blank.getPopulatedFieldsAsMap().containsKey(field);
  System.debug('CAPTURE|'+obj+'.'+field+'|before='+before+'|value='+JSON.serialize(value)+'|after='+after+'|written='+written+'|loaded='+JSON.serialize(loaded.get(field))+'|loadedPop='+loaded.getPopulatedFieldsAsMap().containsKey(field));
  } catch(Exception e) { System.debug('CAPTURE|'+obj+'.'+field+'|throw='+e.getTypeName()); }
 }
}
