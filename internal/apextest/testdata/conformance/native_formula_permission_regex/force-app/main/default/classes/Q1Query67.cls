@IsTest private class Q1Query67 {
 @IsTest static void capture() {
  Account a=new Account(Name='q1');insert a;
  Account b=[SELECT Permission_Formula__c FROM Account WHERE Id=:a.Id];
  Integer n=[SELECT COUNT() FROM Account WHERE Id=:a.Id AND Permission_Formula__c=true];
  System.assert(false,'Q1CALC|apex='+FeatureManagement.checkPermission('Probe_Bypass')+';field='+b.Permission_Formula__c+';filtered='+n);
 }
}
