public class Case349Probe {
 public static String equalityProbe() {
  insert new EqualityProbe__c(Key__c=1.25);
  EqualityProbe__c a=[SELECT Id FROM EqualityProbe__c WHERE Key__c=1.25 LIMIT 1];
  EqualityProbe__c b=new EqualityProbe__c(Key__c=2.50);
  Database.UpsertResult r=Database.upsert(b,EqualityProbe__c.Fields.Key__c,false);
  EqualityProbe__c c=new EqualityProbe__c(Key__c=1.25);
  Database.UpsertResult s=Database.upsert(c,EqualityProbe__c.Fields.Key__c,false);
  insert new EqualityProbe__c(Key__c=3.75);
  EqualityProbe__c d=new EqualityProbe__c(Key__c=4.00);
  Database.UpsertResult t=Database.upsert(d,EqualityProbe__c.Fields.Key__c,false);
  insert new EqualityProbe__c(Key__c=8.00); insert new EqualityProbe__c(Key__c=8.00);
  Database.UpsertResult reject=Database.upsert(new EqualityProbe__c(Key__c=8.00),EqualityProbe__c.Fields.Key__c,false);
  return 'DISTINCT='+r.isSuccess()+','+r.isCreated()+','+(a.Id==b.Id)+';EQUAL='+s.isSuccess()+','+s.isCreated()+','+(a.Id==c.Id)+';MULTI='+t.isSuccess()+','+t.isCreated()+';REJECT='+reject.isSuccess()+','+(reject.isSuccess()?'none':String.valueOf(reject.getErrors()[0].getStatusCode()));
 }
 public static String accountProbe() {
  String out='';
  List<Object> values=new List<Object>{false,true,null,'false','   '};
  for(Integer i=0;i<values.size();i++) {
   try { Account a=new Account(Name='Business '+i); a.put('PersonDoNotCall',values[i]); Database.SaveResult r=Database.insert(a,false); out+='B'+i+'='+r.isSuccess()+','+(r.isSuccess()?String.valueOf([SELECT IsPersonAccount FROM Account WHERE Id=:a.Id].IsPersonAccount):String.valueOf(r.getErrors()[0].getStatusCode()))+';'; }
   catch(Exception e) {out+='B'+i+'=EXCEPTION,'+e.getTypeName()+';';}
  }
  for(Integer i=0;i<5;i++) {
   try { Account a=new Account(Name='BusinessEmail '+i); a.put('PersonEmail',new List<String>{null,'   ','person@example.test','false',' \t '}[i]); Database.SaveResult r=Database.insert(a,false); out+='E'+i+'='+r.isSuccess()+','+(r.isSuccess()?String.valueOf([SELECT IsPersonAccount FROM Account WHERE Id=:a.Id].IsPersonAccount):String.valueOf(r.getErrors()[0].getStatusCode()))+';'; }
   catch(Exception e) {out+='E'+i+'=EXCEPTION,'+e.getTypeName()+';';}
  }
  return out;
 }
}
