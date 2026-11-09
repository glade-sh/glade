public class B3CaseMerge {
 public static String run() {String u=String.valueOf(Crypto.getRandomInteger());Case row=new Case(Subject='Synthetic',Status='New',Origin='Web',SuppliedEmail='s'+u+'@example.invalid');Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=true;Database.SaveResult r=Database.insert(row,o);List<Task> tasks=[SELECT Subject FROM Task WHERE WhatId=:row.Id];return 'case_merge='+r.isSuccess()+','+tasks.size()+','+(!tasks.isEmpty() && tasks[0].Subject=='Email: Response Synthetic');}
}
