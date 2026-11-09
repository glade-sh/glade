public class B3AssignmentEmail {
 public static String run() {
  String u=String.valueOf(Crypto.getRandomInteger());List<String> out=new List<String>();Id queueId=[SELECT Id FROM Group WHERE DeveloperName='SyntheticRouting' AND Type='Queue' LIMIT 1].Id;
  Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerUserEmail=true;o.EmailHeader.triggerOtherEmail=true;o.EmailHeader.triggerAutoResponseEmail=true;o.AssignmentRuleHeader.useDefaultRule=true;
  Lead l=new Lead(LastName='Synthetic'+u,Company='Synthetic',Email='s'+u+'@example.invalid');Database.SaveResult a=Database.insert(l,o);Id leadOwner=[SELECT OwnerId FROM Lead WHERE Id=:l.Id].OwnerId;out.add('lead_assigned='+a.isSuccess()+','+(leadOwner==queueId)+','+[SELECT COUNT() FROM Task WHERE WhoId=:l.Id]);
  Contact c=new Contact(LastName='Contact'+u,Email='c'+u+'@example.invalid');insert c;
  Case row=new Case(Subject='Synthetic',Status='New',Origin='Web',ContactId=c.Id,SuppliedEmail='r'+u+'@example.invalid');Database.SaveResult b=Database.insert(row,o);Id caseOwner=[SELECT OwnerId FROM Case WHERE Id=:row.Id].OwnerId;out.add('case_assigned='+b.isSuccess()+','+(caseOwner==queueId)+','+[SELECT COUNT() FROM Task WHERE WhatId=:row.Id]);
  return String.join(out,'|');
 }
}
