public class OwnedRelProbe {
 public static String observe() {
  Account parent = new Account(Name='Owned parent'); insert parent;
  List<Object> results = new List<Object>();
  for (Boolean populated : new List<Boolean>{true,false}) {
   Contact row = new Contact(LastName='Owned contact'); if (populated) row.AccountId = parent.Id;
   Database.SaveResult outcome = Database.insert(row, false);
   Map<String,Object> item = new Map<String,Object>{'populated'=>populated,'success'=>outcome.isSuccess()};
   if (outcome.isSuccess()) { Contact saved = [SELECT Other__c FROM Contact WHERE Id = :row.Id WITH SYSTEM_MODE]; item.put('same',saved.Other__c==parent.Id); item.put('null',saved.Other__c==null); }
   else { item.put('status',String.valueOf(outcome.getErrors()[0].getStatusCode())); item.put('message',outcome.getErrors()[0].getMessage()); }
   results.add(item);
  }
  return JSON.serialize(results);
 }
}
