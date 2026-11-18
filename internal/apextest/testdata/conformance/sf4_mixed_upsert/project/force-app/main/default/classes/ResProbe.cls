public class ResProbe {
 public static List<String> seen = new List<String>();
 public static String run() {
  Par__c p = new Par__c(Name='Mixed'); insert p;
  Chi__c old = new Chi__c(Name='old', ExtId__c='old', Parent__c=p.Id, Amount__c=10); insert old;
  seen.clear();
  upsert new List<Chi__c>{new Chi__c(ExtId__c='old', Name='updated', Parent__c=p.Id, Amount__c=20), new Chi__c(ExtId__c='new', Name='new', Parent__c=p.Id, Amount__c=7)} Chi__c.ExtId__c;
  Par__c q = [SELECT Cnt__c, SumAmt__c FROM Par__c WHERE Id=:p.Id];
  return 'seen='+String.join(seen,';')+'|final='+q.Cnt__c+','+q.SumAmt__c;
 }
}
