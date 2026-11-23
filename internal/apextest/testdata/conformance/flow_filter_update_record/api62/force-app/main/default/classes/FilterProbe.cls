public class FilterProbe {
 public static String observe() {
  Account a = new Account(Name='Probe null', Amt__c=10);
  Account b = new Account(Name='Probe set', Amt__c=10, Src__c=5);
  insert as system a; insert as system b;
  a = [SELECT Amt__c FROM Account WHERE Id = :a.Id WITH SYSTEM_MODE];
  b = [SELECT Amt__c FROM Account WHERE Id = :b.Id WITH SYSTEM_MODE];
  return 'srcNull=' + a.Amt__c + ';srcSet=' + b.Amt__c;
 }
}
