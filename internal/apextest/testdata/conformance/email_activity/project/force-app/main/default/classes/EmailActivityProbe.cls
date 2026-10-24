public class EmailActivityProbe {
 public static Map<String,Object> observe(String shape) {
  Account parent=new Account(Name='Activity Parent'); insert parent;
  Contact target=new Contact(LastName='Recipient',Email='recipient@example.test',AccountId=parent.Id); insert target;
  Messaging.SingleEmailMessage message=makeMessage(target.Id,parent.Id);
  if(shape=='disabled') message.setSaveAsActivity(false);
  if(shape=='missingWhat') message.setWhatId(null);
  if(shape=='userWhat'||shape=='disabledUserWhat') message.setWhatId(UserInfo.getUserId());
  if(shape=='disabledUserWhat') message.setSaveAsActivity(false);
  if(shape=='contactWhat') message.setWhatId(target.Id);
  if(shape=='missingReference') message.setWhatId('001000000000999AAA');
  if(shape=='deletedWhat') { Account deleted=new Account(Name='Deleted Parent');insert deleted;Id deletedId=deleted.Id;delete deleted;message.setWhatId(deletedId); }
  if(shape=='addressOnly') {message.setTargetObjectId(null);message.setToAddresses(new List<String>{'recipient@example.test'});}
  if(shape=='detachedTarget') {message.setTreatTargetObjectAsRecipient(false);message.setToAddresses(new List<String>{'recipient@example.test'});}
  Map<String,Object> result=new Map<String,Object>();
  if(shape=='template') {
   EmailTemplate template=[SELECT Id FROM EmailTemplate WHERE DeveloperName='ActivityTemplate' LIMIT 1];
   message=Messaging.renderStoredEmailTemplate(template.Id,target.Id,parent.Id);
   result.put('renderTasks',[SELECT COUNT() FROM Task WHERE WhoId=:target.Id]);
   message.setSaveAsActivity(true);
  }
  Savepoint point=Database.setSavepoint();
  List<Messaging.SingleEmailMessage> messages=new List<Messaging.SingleEmailMessage>{message};
  if(shape=='bulkSend'||shape=='partial'||shape=='allOrNone') {
   Messaging.SingleEmailMessage second=makeMessage(target.Id,parent.Id);
   second.setSubject('Second activity');
   if(shape!='bulkSend') second.setWhatId(UserInfo.getUserId());
   messages.add(second);
  }
  try {
   Messaging.SendEmailResult[] sent=Messaging.sendEmail(messages,shape=='allOrNone');
   List<Boolean> success=new List<Boolean>();List<Integer> errors=new List<Integer>();List<String> codes=new List<String>();List<String> errorFields=new List<String>();List<String> errorMessages=new List<String>();
   for(Messaging.SendEmailResult item:sent) {
    success.add(item.isSuccess());errors.add(item.getErrors().size());
    for(Messaging.SendEmailError issue:item.getErrors()) {codes.add(String.valueOf(issue.getStatusCode()));List<String> fields=issue.getFields();if(fields.size()==2&&fields[0]=='whatId') fields[1]='<whatId>';errorFields.add(JSON.serialize(fields));errorMessages.add(issue.getMessage());}
   }
   result.put('sent',success);result.put('errors',errors);result.put('codes',codes);result.put('errorFields',errorFields);result.put('errorMessages',errorMessages);
  } catch(Exception issue) {result.put('exception',issue.getTypeName());}
  List<Task> tasks=[SELECT Subject,Description,Status,Priority,TaskSubtype,WhoId,WhatId,OwnerId,ActivityDate,IsClosed FROM Task WHERE WhoId=:target.Id ORDER BY Subject];
  result.put('tasks',tasks.size());
  List<Object> activity=new List<Object>();
  for(Task task:tasks) activity.add(new Map<String,Object>{'subject'=>task.Subject,'description'=>task.Description,'status'=>task.Status,'priority'=>task.Priority,'subtype'=>task.TaskSubtype,'who'=>task.WhoId==target.Id,'what'=>task.WhatId==parent.Id,'owner'=>task.OwnerId==UserInfo.getUserId(),'date'=>task.ActivityDate==Date.today(),'closed'=>task.IsClosed});
  result.put('activity',activity);
  if(shape=='rollback') {Database.rollback(point);result.put('afterRollback',[SELECT COUNT() FROM Task WHERE WhoId=:target.Id]);}
  return result;
 }
 private static Messaging.SingleEmailMessage makeMessage(Id who,Id what) {
  Messaging.SingleEmailMessage message=new Messaging.SingleEmailMessage();message.setTargetObjectId(who);message.setWhatId(what);message.setSubject('Synthetic activity');message.setPlainTextBody('Synthetic content');message.setSaveAsActivity(true);return message;
 }
}
