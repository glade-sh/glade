public class B3Review {
 public static String run() {
  Contact contact=new Contact(LastName='B3Valid',Email='valid@example.invalid');insert contact;
  Database.DMLOptions email=new Database.DMLOptions();email.EmailHeader.triggerOtherEmail=true;
  Case populate=new Case(Subject='B3Populate',Origin='Web',Status='New');
  Case clear=new Case(Subject='B3Clear',Origin='Web',Status='New',ContactId=contact.Id);
  Database.SaveResult a=Database.insert(populate,email),b=Database.insert(clear,email);
  String first='populate='+a.isSuccess()+(a.isSuccess()?'':','+a.getErrors()[0].getStatusCode());
  String second='clear='+b.isSuccess()+(b.isSuccess()?'':','+b.getErrors()[0].getStatusCode());
  Lead bad=new Lead(Id='00Q000000000001AAA',LastName='Bad',Company='Synthetic',Email='bad@example.invalid');
  Lead good=new Lead(LastName='Good',Company='Synthetic',Email='good@example.invalid');
  Database.DMLOptions response=new Database.DMLOptions();response.EmailHeader.triggerAutoResponseEmail=true;response.optAllOrNone=false;
  try {Database.SaveResult[] rs=Database.insert(new List<Lead>{bad,good},response);return first+'|'+second+'|partial='+rs[0].isSuccess()+','+rs[1].isSuccess()+','+rs[0].getErrors()[0].getStatusCode()+','+[SELECT COUNT() FROM Task WHERE WhoId=:good.Id];}
  catch(Exception e){return first+'|'+second+'|partial=exception:'+e.getTypeName();}
 }
}
