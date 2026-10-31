public class MultiHopProbe {
 public static String label(SObject row) { SObject a=row.getSObject('Account'); if(a==null) return 'null account'; SObject p=a.getSObject('Parent'); if(p==null) return 'null parent'; return JSON.serialize(p.get('result')); }
 public static Map<String,Map<String,String>> observe() {
  Savepoint sp=Database.setSavepoint();
  Account t=new Account(Name='Owned Parent T',Industry='Technology'); Account b=new Account(Name='Owned Parent B',Industry='Banking');insert new List<Account>{t,b};
  Account ct=new Account(Name='Owned Child T',ParentId=t.Id);Account cb=new Account(Name='Owned Child B',ParentId=b.Id);insert new List<Account>{ct,cb};
  insert new List<Contact>{new Contact(LastName='Owned T',AccountId=ct.Id),new Contact(LastName='Owned B',AccountId=cb.Id)};
  Map<String,String> queries=new Map<String,String>{
   'directTechnology'=>'SELECT toLabel(Account.Parent.Industry) result FROM Contact WHERE LastName=\'Owned T\' AND toLabel(Account.Parent.Industry)=\'Technologie\'',
   'directBanking'=>'SELECT toLabel(Account.Parent.Industry) result FROM Contact WHERE LastName=\'Owned B\' AND toLabel(Account.Parent.Industry)=\'Banque FR\'',
   'qualified'=>'SELECT toLabel(Contact.Account.Parent.Industry) result FROM Contact WHERE LastName=\'Owned T\' AND toLabel(Contact.Account.Parent.Industry)=\'Technologie\'',
   'missing'=>'SELECT toLabel(Account.Parent.Industry) result FROM Contact WHERE LastName=\'Owned T\' AND toLabel(Account.Parent.Industry)=\'No Match\'',
   'locator'=>'SELECT toLabel(Account.Parent.Industry) result FROM Contact WHERE LastName=\'Owned T\'',
   'child'=>'SELECT (SELECT toLabel(Account.Parent.Industry) result FROM Contacts) FROM Account WHERE Name=\'Owned Child T\''
  };
  Map<String,Map<String,String>> rows=new Map<String,Map<String,String>>();
  for(String shape:queries.keySet()) { try { List<SObject> found;
   if(shape=='locator') {Database.QueryLocatorIterator it=Database.getQueryLocator(queries.get(shape)).iterator();found=new List<SObject>();while(it.hasNext()) found.add(it.next());} else found=Database.query(queries.get(shape));
   List<Object> values=new List<Object>();for(SObject x:found){if(shape=='child') {for(SObject child:x.getSObjects('Contacts')) values.add(label(child));}else values.add(label(x));}
   rows.put(shape,new Map<String,String>{'value'=>JSON.serialize(values)});
  } catch(Exception e) {rows.put(shape,new Map<String,String>{'value'=>e.getTypeName()+': '+e.getMessage()});} }
  Database.rollback(sp);return rows;
 }
 public static void emit() {Map<String,Map<String,String>> rows=observe();for(String k:rows.keySet()) System.debug('MULTIHOP_LABEL|'+k+'|'+JSON.serialize(rows.get(k)));}
}
