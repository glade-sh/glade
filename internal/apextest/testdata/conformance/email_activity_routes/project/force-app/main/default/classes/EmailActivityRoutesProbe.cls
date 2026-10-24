public class EmailActivityRoutesProbe {
 public static Map<String,Object> observe(String shape) {
  if(shape.startsWith('flow')) return flowObservation(shape);
  ActivityParent__c parent=new ActivityParent__c(Name='Custom Parent');insert parent;
  Contact target=new Contact(LastName='Recipient',Email=UserInfo.getUserEmail());insert target;
  Id related=parent.Id;
  if(shape.startsWith('noActivities')) {PlainParent__c plain=new PlainParent__c(Name='Plain Parent');insert plain;related=plain.Id;}
  if(shape.startsWith('deleted')) delete parent;
  if(shape.startsWith('missing')) related=String.valueOf(parent.Id).substring(0,3)+'000000000999';
  Messaging.SingleEmailMessage message=new Messaging.SingleEmailMessage();
  message.setTargetObjectId(target.Id);message.setWhatId(related);message.setSubject('Custom activity');message.setPlainTextBody('Custom body');message.setSaveAsActivity(!shape.startsWith('disabled'));
  Savepoint point=Database.setSavepoint();
  Messaging.SendEmailResult[] sent;
  try {
  if(shape.endsWith('Default')) sent=Messaging.sendEmail(new Messaging.SingleEmailMessage[]{message});
  else sent=Messaging.sendEmail(new Messaging.SingleEmailMessage[]{message},false);
  } catch(Exception issue) {return new Map<String,Object>{'exception'=>issue.getTypeName(),'tasks'=>[SELECT COUNT() FROM Task WHERE WhoId=:target.Id]};}
  List<String> codes=new List<String>();for(Messaging.SendEmailError issue:sent[0].getErrors()) codes.add(String.valueOf(issue.getStatusCode()));
  List<Task> tasks=[SELECT WhatId,WhoId,Subject FROM Task WHERE WhoId=:target.Id];
  Map<String,Object> result=new Map<String,Object>{'sent'=>sent[0].isSuccess(),'errors'=>sent[0].getErrors().size(),'codes'=>codes,'tasks'=>tasks.size()};
  if(!tasks.isEmpty()) {result.put('whatMatches',tasks[0].WhatId==related);result.put('whoMatches',tasks[0].WhoId==target.Id);result.put('subject',tasks[0].Subject);}
  if(shape.startsWith('rollback')) {Database.rollback(point);result.put('afterRollback',[SELECT COUNT() FROM Task WHERE WhoId=:target.Id]);}
  return result;
 }
 private static Map<String,Object> flowObservation(String shape) {
  String setting=shape.endsWith('Omitted')?'Omitted':(shape.endsWith('False')?'False':'True');
  String subject='Activity '+shape;
  if(shape.startsWith('flowTrigger')) insert new Account(Name=subject,Description=UserInfo.getUserEmail());
  else {Account parent=new Account(Name='Flow parent');insert parent;Flow.Interview interview=Flow.Interview.createInterview('ActivityEmail'+setting,new Map<String,Object>{'Address'=>UserInfo.getUserEmail(),'Body'=>'Flow body','RelatedId'=>parent.Id,'Subject'=>subject});interview.start();}
  return new Map<String,Object>{'messages'=>[SELECT COUNT() FROM EmailMessage WHERE Subject=:subject],'tasks'=>[SELECT COUNT() FROM Task]};
 }
}
