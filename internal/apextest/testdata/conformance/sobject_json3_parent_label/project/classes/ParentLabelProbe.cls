public class ParentLabelProbe {
 public static String read(SObject row, String key) { try { return JSON.serialize(row.get(key)); } catch(Exception e) { return e.getTypeName(); } }
 public static Map<String,Map<String,String>> observe() {
  Savepoint sp=Database.setSavepoint();
  Account a=new Account(Name='Owned Agriculture',Industry='Agriculture'); Account b=new Account(Name='Owned Banking',Industry='Banking'); Account c=new Account(Name='Owned Technology',Industry='Technology'); Account n=new Account(Name='Owned Empty'); insert new List<Account>{a,b,c,n};
  insert new List<Contact>{new Contact(LastName='Owned A',AccountId=a.Id),new Contact(LastName='Owned B',AccountId=b.Id),new Contact(LastName='Owned T',AccountId=c.Id),new Contact(LastName='Owned N',AccountId=n.Id),new Contact(LastName='Owned Missing')};
  Map<String,String> names=new Map<String,String>{'parentTechnology'=>'Owned T','parentBanking'=>'Owned B','parentAgriculture'=>'Owned A','parentNull'=>'Owned N','parentUnaliased'=>'Owned T','parentMissing'=>'Owned Missing','rootReject'=>'Owned T'};
  Map<String,Map<String,String>> rows=new Map<String,Map<String,String>>();
  for(String shape:names.keySet()) { try {
   String who=names.get(shape); String expr=shape=='parentUnaliased'?'toLabel(Account.Industry)':'toLabel(Account.Industry) result';
   SObject row=Database.query('SELECT '+expr+' FROM Contact WHERE LastName = :who')[0]; SObject parent=row.getSObject('Account');
   if(shape=='rootReject') {rows.put(shape,new Map<String,String>{'value'=>read(row,'result')});continue;}
   if(parent==null) {rows.put(shape,new Map<String,String>{'value'=>'null parent'});continue;}
   Map<String,Object> serialized=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(parent)); serialized.remove('attributes'); serialized.remove('Id');
   List<String> values=new List<String>{read(parent,'result'),read(parent,'Industry'),JSON.serialize(serialized)};
   rows.put(shape,new Map<String,String>{'value'=>JSON.serialize(values)});
  } catch(Exception e) { rows.put(shape,new Map<String,String>{'value'=>e.getTypeName()+': '+e.getMessage()}); } }
  Database.rollback(sp); return rows;
 }
 public static void emit() { Map<String,Map<String,String>> rows=observe(); for(String k:rows.keySet()) System.debug('PARENT_LABEL|'+k+'|'+JSON.serialize(rows.get(k))); }
}
