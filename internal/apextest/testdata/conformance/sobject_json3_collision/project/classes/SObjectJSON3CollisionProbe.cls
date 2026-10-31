public class SObjectJSON3CollisionProbe {
 public static Map<String,Map<String,String>> observe() {
  Savepoint sp=Database.setSavepoint();
  Account a=new Account(Name='One',Industry='Agriculture'); Account b=new Account(Name='Two',Industry='Banking'); Account c=new Account(Name='Three',Industry='Technology'); Account n=new Account(Name='Null'); insert new List<Account>{a,b,c,n};
  Map<String,Id> ids=new Map<String,Id>{'industryA'=>a.Id,'industryB'=>b.Id,'industryC'=>c.Id,'nullBoundary'=>n.Id,'queryRejection'=>a.Id};
  Map<String,Map<String,String>> rows=new Map<String,Map<String,String>>();
  for(String shape:ids.keySet()) {
   try {
    Id id=ids.get(shape);
    if (shape=='queryRejection') { Database.query('SELECT Unknown_Field__c FROM Account WHERE Id=:id'); rows.put(shape,new Map<String,String>{'value'=>'no exception'}); continue; }
    SObject aliasRow=Database.query('SELECT toLabel(Industry) result FROM Account WHERE Id=:id')[0];
    SObject rawRow=Database.query('SELECT Industry FROM Account WHERE Id=:id')[0];
    SObject unaliased=Database.query('SELECT toLabel(Industry) FROM Account WHERE Id=:id')[0];
    rows.put(shape,new Map<String,String>{'value'=>JSON.serialize(new List<Object>{rawRow.get('Industry'),aliasRow.get('result'),unaliased.get('Industry')})});
   } catch(Exception e) {rows.put(shape,new Map<String,String>{'value'=>e.getTypeName()});}
  }
  Database.rollback(sp); return rows;
 }
 public static void emit() {Map<String,Map<String,String>> rows=observe(); for(String k:rows.keySet()) System.debug('SOBJECT_COLLISION|'+k+'|'+JSON.serialize(rows.get(k)));}
}
