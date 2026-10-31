public class TypeofErrorProbe {
 public static Map<String,Map<String,String>> observe() {
  Map<String,String> queries = new Map<String,String>{
   'ownerAccount'=>'SELECT TYPEOF Owner WHEN Account THEN Name END FROM Task LIMIT 1',
   'ownerContact'=>'SELECT TYPEOF Owner WHEN Contact THEN Name END FROM Task LIMIT 1',
   'ownerOpportunity'=>'SELECT TYPEOF Owner WHEN Opportunity THEN Name END FROM Task LIMIT 1',
   'whoAccount'=>'SELECT TYPEOF Who WHEN Account THEN Name END FROM Task LIMIT 1',
   'validBoundary'=>'SELECT TYPEOF Owner WHEN User THEN Name END FROM Task LIMIT 1',
   'unknownType'=>'SELECT TYPEOF Owner WHEN NotARealType__c THEN Name END FROM Task LIMIT 1'
  };
  Map<String,Map<String,String>> rows = new Map<String,Map<String,String>>();
  for (String shape:queries.keySet()) {
   String value;
   try { Database.query(queries.get(shape)); value='ok'; }
   catch (Exception e) { value=e.getTypeName()+': '+e.getMessage(); }
   rows.put(shape,new Map<String,String>{'value'=>value});
  }
  return rows;
 }
 public static void emit() { for (String k:observe().keySet()) System.debug('SOBJECT_TYPEOF|'+k+'|'+JSON.serialize(observe().get(k))); }
}
