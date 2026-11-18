public class PeSubscriber {
 public static Integer seq=0;
 public static void receive(String channel, List<SObject> events) {
  seq++;
  List<Account> rows=new List<Account>();
  for (SObject item: events) {
   String payload=(String)item.get('Payload__c');
   rows.add(new Account(Name='PE-'+payload, Description=channel+'|batch'+seq+'|size'+events.size()));
  }
  insert rows;
 }
}
