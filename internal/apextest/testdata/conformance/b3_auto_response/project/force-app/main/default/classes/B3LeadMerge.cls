public class B3LeadMerge {
 public static String run() {
  String u=String.valueOf(Crypto.getRandomInteger());Lead row=new Lead(LastName='Synthetic'+u,Company='Synthetic',Email='s'+u+'@example.invalid');Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=true;Database.SaveResult r=Database.insert(row,o);
  List<Task> tasks=[SELECT Subject FROM Task WHERE WhoId=:row.Id];return 'lead_merge='+r.isSuccess()+','+tasks.size()+','+(!tasks.isEmpty() && tasks[0].Subject=='Email: Response Synthetic'+u);
 }
}
