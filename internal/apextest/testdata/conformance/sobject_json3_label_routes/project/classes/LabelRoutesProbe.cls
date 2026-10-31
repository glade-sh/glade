public class LabelRoutesProbe {
 public static Map<String,Map<String,String>> observe() {
  Savepoint sp=Database.setSavepoint();
  Account a=new Account(Name='Owned Agriculture',Industry='Agriculture'); Account b=new Account(Name='Owned Banking',Industry='Banking'); Account c=new Account(Name='Owned Technology',Industry='Technology'); Account n=new Account(Name='Owned Empty'); insert new List<Account>{a,b,c,n};
  insert new List<Contact>{new Contact(LastName='Owned T',AccountId=c.Id),new Contact(LastName='Owned B',AccountId=b.Id),new Contact(LastName='Owned N',AccountId=n.Id)};
  Map<String,String> queries=new Map<String,String>{
   'filterTechnology'=>'SELECT Name FROM Account WHERE Name LIKE \'Owned %\' AND toLabel(Industry) = \'Technologie\' ORDER BY Name',
   'filterBanking'=>'SELECT Name FROM Account WHERE Name LIKE \'Owned %\' AND toLabel(Industry) = \'Banque FR\' ORDER BY Name',
   'filterAgriculture'=>'SELECT Name FROM Account WHERE Name LIKE \'Owned %\' AND toLabel(Industry) = \'Agriculture FR\' ORDER BY Name',
   'filterMaster'=>'SELECT Name FROM Account WHERE Name LIKE \'Owned %\' AND toLabel(Industry) = \'Technology\' ORDER BY Name',
   'filterMissing'=>'SELECT Name FROM Account WHERE Name LIKE \'Owned %\' AND toLabel(Industry) = \'Never\' ORDER BY Name',
   'filterLike'=>'SELECT Name FROM Account WHERE Name LIKE \'Owned %\' AND toLabel(Industry) LIKE \'Tech%\' ORDER BY Name',
   'parentTechnology'=>'SELECT toLabel(Account.Industry) result FROM Contact WHERE LastName = \'Owned T\'',
   'parentBanking'=>'SELECT toLabel(Account.Industry) result FROM Contact WHERE LastName = \'Owned B\'',
   'parentNull'=>'SELECT toLabel(Account.Industry) result FROM Contact WHERE LastName = \'Owned N\''
  };
  Map<String,Map<String,String>> rows=new Map<String,Map<String,String>>();
  for(String shape:queries.keySet()) { try { List<SObject> found=Database.query(queries.get(shape)); List<Object> result=new List<Object>(); for(SObject x:found) result.add(x.get(shape.startsWith('parent')?'result':'Name')); rows.put(shape,new Map<String,String>{'value'=>JSON.serialize(result)}); } catch(Exception e) { rows.put(shape,new Map<String,String>{'value'=>e.getTypeName()+': '+e.getMessage()}); } }
  Database.rollback(sp); return rows;
 }
 public static void emit() { Map<String,Map<String,String>> rows=observe(); for(String k:rows.keySet()) System.debug('LABEL_ROUTES|'+k+'|'+JSON.serialize(rows.get(k))); }
}
