public class B3LeadDetails {
 public static String run() {
  String u=String.valueOf(Crypto.getRandomInteger());Lead row=new Lead(LastName='Synthetic'+u,Company='Synthetic',Email='s'+u+'@example.invalid');Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=true;Database.SaveResult r=Database.insert(row,o);
  List<Task> tasks=[SELECT Subject,Status,Priority,TaskSubtype,WhoId,WhatId,IsClosed,ActivityDate FROM Task WHERE WhoId=:row.Id];
  if(tasks.isEmpty())return 'lead_details='+r.isSuccess()+',0';Task t=tasks[0];return 'lead_details='+r.isSuccess()+','+tasks.size()+','+t.Subject+','+t.Status+','+t.Priority+','+t.TaskSubtype+','+(t.WhoId==row.Id)+','+(t.WhatId==null)+','+t.IsClosed+','+(t.ActivityDate==Date.today());
 }
}
