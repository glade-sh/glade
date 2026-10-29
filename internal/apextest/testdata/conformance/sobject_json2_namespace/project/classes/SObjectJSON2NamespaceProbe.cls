public class SObjectJSON2NamespaceProbe {
 public static Map<String,Map<String,String>> observe() {
  Savepoint sp=Database.setSavepoint();
  JSON2_Probe_20261026__c row=new JSON2_Probe_20261026__c(Name='Owned',Value__c='value');insert row;
  Id rowId=row.Id;
  Map<String,Object> values=new Map<String,Object>{'constructed'=>new JSON2_Probe_20261026__c(Value__c='value',Name='Owned'), 'queried'=>Database.query('SELECT Value__c,Name FROM JSON2_Probe_20261026__c WHERE Id=:rowId')[0], 'empty'=>Database.query('SELECT Name FROM JSON2_Probe_20261026__c WHERE Name=\'Missing synthetic row\'')};
  Map<String,Map<String,String>> rows=new Map<String,Map<String,String>>();
  for(String key:values.keySet()) {
   Object value=values.get(key);Map<String,String> out=new Map<String,String>();
   out.put('serialize',JSON.serialize(value));out.put('pretty',JSON.serializePretty(value));out.put('suppress',JSON.serialize(value,true));out.put('prettySuppress',JSON.serializePretty(value,true));
   if(key!='empty') {try{out.put('roundTrip',JSON.serialize(JSON.deserialize(JSON.serialize(value),SObject.class)));}catch(Exception e){out.put('roundTrip',e.getTypeName()+': '+e.getMessage());}}
   for(String op:out.keySet()) out.put(op,out.get(op).replace(String.valueOf(rowId),'<record>'));
   rows.put(key,out);
  }
  try {Database.query('SELECT Missing__c FROM JSON2_Probe_20261026__c');} catch(Exception e){rows.put('rejection',new Map<String,String>{'query'=>e.getTypeName()+': '+e.getMessage()});}
  Database.rollback(sp);return rows;
 }
 public static void emit(){Map<String,Map<String,String>> rows=observe();for(String shape:rows.keySet()) System.debug(LoggingLevel.ERROR,'SOBJECT_JSON2_NAMESPACE|'+shape+'|'+JSON.serialize(rows.get(shape)));}
}
