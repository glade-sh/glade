public class CreatedIdProbe {
 public static String observe() {
  Account a=new Account(Name='Probe parent'); Database.SaveResult r=Database.insert(a,false);
  if (!r.isSuccess()) return 'error:'+String.valueOf(r.getErrors()[0].getStatusCode());
  Account saved=[SELECT ProbeParent__c FROM Account WHERE Id=:a.Id WITH SYSTEM_MODE];
  return 'linked='+(saved.ProbeParent__c!=null);
 }
}
