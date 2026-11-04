  Map<String,String> out = new Map<String,String>();
  for (String pair : new List<String>{'Asset.StatusReason','Contact.GenderIdentity','Contact.Pronouns','Contract.ShippingGeocodeAccuracy','Event.Type','Lead.GenderIdentity','Lead.Pronouns','Task.Type'}) {
   List<String> bits = pair.split('\\.');
   try { Schema.SObjectType ot = Schema.getGlobalDescribe().get(bits[0]); Schema.SObjectField f = ot.getDescribe().fields.getMap().get(bits[1]); List<String> vals = new List<String>(); for (Schema.PicklistEntry v : f.getDescribe().getPicklistValues()) vals.add(v.getValue()+':'+v.isActive()+':'+v.isDefaultValue()); out.put(pair, String.join(vals,'~')); } catch (Exception e) { out.put(pair,'ERROR:'+e.getTypeName()); }
  }

System.debug('CASE251START' + JSON.serialize(out) + 'CASE251END');
System.debug('CASE251BOUNDARY:' + Schema.SObjectType.Lead.fields.getMap().get('Id').getDescribe().getPicklistValues().size());
System.debug('CASE251REJECT:' + Schema.SObjectType.Lead.fields.getMap().containsKey('Case251DefinitelyMissing__c'));
