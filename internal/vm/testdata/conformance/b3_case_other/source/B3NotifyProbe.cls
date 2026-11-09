public class B3NotifyProbe {
 public static String run() {
  String unique=String.valueOf(Crypto.getRandomInteger());
  Contact c=new Contact(LastName='Synthetic'+unique,Email='synthetic'+unique+'@example.invalid');insert c;
  Case row=new Case(Subject='Synthetic',Status='New',Origin='Phone',ContactId=c.Id);
  Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerOtherEmail=true;
  Database.SaveResult r=Database.insert(row,o);
  List<Task> tasks=[SELECT Subject,Status,TaskSubtype FROM Task WHERE WhatId=:row.Id];
  Integer messages=[SELECT COUNT() FROM EmailMessage WHERE RelatedToId=:row.Id];
  String detail=tasks.isEmpty()?'none':tasks[0].Subject+'~'+tasks[0].Status+'~'+tasks[0].TaskSubtype;
  return r.isSuccess()+','+tasks.size()+','+messages+','+detail;
 }
}
