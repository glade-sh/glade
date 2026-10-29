public class ResidualBoundaryProbe {
 public class RecordFactory {}
 public static Map<String,Object> recordIdentity() {
  Account row=new Account(Name='Owned boundary');insert row;
  return new Map<String,Object>{'factory'=>RecordFactory.class.getName(),'record'=>row.Id.getSObjectType().getDescribe().getName()};
 }
 public static Map<String,Object> emailActivity(Boolean automatic,Boolean manual) {
  Account parent=new Account(Name='Owned activity');insert parent;
  Contact recipient=new Contact(LastName='Owned',Email='owned@example.invalid',AccountId=parent.Id);insert recipient;
  EmailTemplate letter=[SELECT Id FROM EmailTemplate WHERE DeveloperName='BoundaryLetter' LIMIT 1];
  Messaging.SingleEmailMessage message=Messaging.renderStoredEmailTemplate(letter.Id,recipient.Id,parent.Id);
  if(!automatic)message.setSaveAsActivity(false);
  Messaging.SendEmailResult[] sent=Messaging.sendEmail(new Messaging.SingleEmailMessage[]{message});
  if(manual)insert new Task(WhoId=recipient.Id,WhatId=parent.Id,Subject='Owned application activity',Status='Completed',Priority='Normal');
  return new Map<String,Object>{'sent'=>sent[0].isSuccess(),'tasks'=>[SELECT COUNT() FROM Task WHERE WhoId=:recipient.Id]};
 }
 public static Integer readTraversal() {
  List<Integer> values=new List<Integer>{4,8};List<Integer> wanted=new List<Integer>{8};Integer matches=0;
  for(Integer target:wanted)for(Integer value:values)if(value==target)matches+=values.indexOf(value)+1;
  return matches;
 }
 public static void mutateTraversal() {
  List<Integer> values=new List<Integer>{4,8};List<Integer> wanted=new List<Integer>{4};
  for(Integer target:wanted)for(Integer value:values)if(value==target)removeEntry(values,values.indexOf(value));
 }
 private static void removeEntry(List<Integer> values,Integer position){values.remove(position);}
}
