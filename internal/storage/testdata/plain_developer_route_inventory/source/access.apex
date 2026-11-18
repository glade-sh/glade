List<String> names=new List<String>{'UserEntityAccess','UserFieldAccess'};
Map<String,Schema.SObjectType> inventory=Schema.getGlobalDescribe();
List<String> results=new List<String>();
results.add('AccountControl='+(Type.forName('Account')!=null));
results.add('MissingControl='+(Type.forName('GladeMissingProbe__c')!=null));
for(String n:names) {
 String typ; try {typ=String.valueOf(Type.forName(n)!=null);} catch(Exception e) {typ=e.getTypeName();}
 results.add(n+':type='+typ);
 results.add(n+':describe='+inventory.containsKey(n));
 String cnt; try {cnt=String.valueOf(Database.countQuery('SELECT count() FROM '+n)>0);} catch(Exception e) {cnt=e.getTypeName();}
 results.add(n+':count='+cnt);
 String rows; try {rows=String.valueOf(!Database.query('SELECT Id FROM '+n+' LIMIT 1').isEmpty());} catch(Exception e) {rows=e.getTypeName();}
 results.add(n+':select='+rows);
}
System.assert(false,'OBS|'+String.join(results,';'));
