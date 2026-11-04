public class Read349Probe {
  public static List<Object> run() {
    Savepoint sp=Database.setSavepoint(); List<Object> out=new List<Object>();
    for (String input : new List<String>{'1.250','1.251','0.000'}) {
      ReadProbe__c r=new ReadProbe__c(Name='Probe',Balance__c=Decimal.valueOf(input)); Database.insert(r,AccessLevel.SYSTEM_MODE);
      ReadProbe__c partial=[SELECT Id FROM ReadProbe__c WHERE Id=:r.Id WITH SYSTEM_MODE];
      Boolean rejected=false;
      try { Decimal ignored=partial.Balance__c; } catch (SObjectException e) { rejected=true; }
      ReadProbe__c selected=[SELECT Balance__c FROM ReadProbe__c WHERE Id=:r.Id WITH SYSTEM_MODE];
      out.add(new Map<String,Object>{'input'=>input,'unqueriedRejected'=>rejected,'selected'=>String.valueOf(selected.Balance__c)});
    }
    Database.rollback(sp); return out;
  }
}
