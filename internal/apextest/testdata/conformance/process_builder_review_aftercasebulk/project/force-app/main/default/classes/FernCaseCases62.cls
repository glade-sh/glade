public class FernCaseCases62 {
 public static String valid(){
  Contact p=new Contact(LastName='person',Email='synthetic@example.invalid'); insert p;
  Case c=new Case(Subject='one',Status='New',Origin='Phone',Fern_Choice__c='A',Fern_Mode__c='A',Fern_Kind__c='ready',Fern_Person__c=p.Id); insert c;String before=FernCaseLog.take();
  c.Status='Working';c.Fern_Ready__c=true; Database.SaveResult r=Database.update(c,false);Case saved=[SELECT Status,Fern_Ready__c FROM Case WHERE Id=:c.Id];return 'api=62.0 | insert='+before+' | update='+r.isSuccess()+':'+saved.Status+':'+saved.Fern_Ready__c+' | log='+FernCaseLog.take();
 }
 public static String nullperson(){
  Case c=new Case(Subject='blank',Status='New',Origin='Phone',Fern_Choice__c='B',Fern_Kind__c='ready');insert c;FernCaseLog.take();c.Status='Working';Database.SaveResult r=Database.update(c,false);return 'api=62.0 | person='+r.isSuccess()+':'+(r.isSuccess()?'none':String.valueOf(r.getErrors()[0].getStatusCode()))+' | log='+FernCaseLog.take();
 }
 public static String nullemail(){
  Contact p=new Contact(LastName='person');insert p;Case c=new Case(Subject='email',Status='New',Origin='Phone',Fern_Choice__c='B',Fern_Kind__c='ready',Fern_Person__c=p.Id);insert c;FernCaseLog.take();c.Status='Working';Database.SaveResult r=Database.update(c,false);return 'api=62.0 | email='+r.isSuccess()+':'+(r.isSuccess()?'none':String.valueOf(r.getErrors()[0].getStatusCode()))+' | log='+FernCaseLog.take();
 }
 public static String changed(){
  Contact p=new Contact(LastName='person',Email='synthetic@example.invalid');insert p;Case c=new Case(Subject='changed',Status='Working',Origin='Phone',Fern_Choice__c='B',Fern_Kind__c='ready',Fern_Mode__c='B',Fern_Person__c=p.Id);insert c;FernCaseLog.take();c.Fern_Ready__c=true;Database.SaveResult r=Database.update(c,false);String first=FernCaseLog.take();c.Subject='same';Database.SaveResult q=Database.update(c,false);return 'api=62.0 | changed='+r.isSuccess()+':'+first+' | unchanged='+q.isSuccess()+':'+FernCaseLog.take();
 }

 public static String cohort62(Boolean reverse, Boolean partial){
  Contact good=new Contact(LastName='good',Email='synthetic@example.invalid');insert good;
  Case a=new Case(Subject='good',Status='New',Origin='Phone',Fern_Choice__c='B',Fern_Kind__c='ready',Fern_Mode__c='A',Fern_Person__c=good.Id);
  Case b=new Case(Subject='bad',Status='New',Origin='Phone',Fern_Choice__c='B',Fern_Kind__c='ready',Fern_Mode__c='A');insert new List<Case>{a,b};FernCaseLog.take();
  a.Status='Working';a.Fern_Ready__c=true;b.Status='Working';b.Fern_Ready__c=true;List<Case> rows=reverse?new List<Case>{b,a}:new List<Case>{a,b};
  List<Database.SaveResult> results=Database.update(rows,partial);List<String> out=new List<String>{'api=62.0'};for(Database.SaveResult r:results)out.add('row='+r.isSuccess()+':'+(r.isSuccess()?'none':String.valueOf(r.getErrors()[0].getStatusCode())));out.add('log='+FernCaseLog.take());return String.join(out,' | ');
 }
 public static String cohort(){return cohort62(false,false);}
 public static String reverse(){return cohort62(true,false);}
}
