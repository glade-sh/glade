public class B3UserNotify {
 public static String run() {
  String u=String.valueOf(Crypto.getRandomInteger());Id q=[SELECT Id FROM Group WHERE DeveloperName='SyntheticRouting' AND Type='Queue' LIMIT 1].Id;Id target=[SELECT Id FROM User WHERE Alias='b3target' LIMIT 1].Id;List<String> out=new List<String>();
  for(Boolean enabled:new List<Boolean>{false,true}) {
   Database.DMLOptions o=new Database.DMLOptions();o.AssignmentRuleHeader.useDefaultRule=true;o.EmailHeader.triggerUserEmail=enabled;
   Lead l=new Lead(LastName='Synthetic'+u+enabled,Company='Synthetic',OwnerId=q);Database.SaveResult a=Database.insert(l,o);Id owner=[SELECT OwnerId FROM Lead WHERE Id=:l.Id].OwnerId;out.add('lead_notify_'+enabled+'='+a.isSuccess()+','+(owner==target)+','+[SELECT COUNT() FROM Task WHERE WhoId=:l.Id]);
   Case c=new Case(Subject='Synthetic',Status='New',Origin='Phone',OwnerId=q);Database.SaveResult b=Database.insert(c,o);Id caseOwner=[SELECT OwnerId FROM Case WHERE Id=:c.Id].OwnerId;out.add('case_notify_'+enabled+'='+b.isSuccess()+','+(caseOwner==target)+','+[SELECT COUNT() FROM Task WHERE WhatId=:c.Id]);
  }return String.join(out,'|');
 }
}
