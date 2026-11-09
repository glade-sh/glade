public class B3CaseOtherProbe {
 public static String run() {
  List<String> out=new List<String>();
  for(Integer i=0;i<3;i++) {
   Id contactId=null;
   if(i>0) { String unique=String.valueOf(Crypto.getRandomInteger());Contact c=new Contact(LastName='Synthetic'+unique,Email=i==2?'synthetic'+unique+'@example.invalid':null);insert c;contactId=c.Id; }
   Case c=new Case(Subject='Synthetic',Status='New',Origin='Phone',ContactId=contactId);
   Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerOtherEmail=true;
   Database.SaveResult r=Database.insert(c,o);
   out.add('case_other_'+i+'='+r.isSuccess()+(r.isSuccess()?'':','+r.getErrors()[0].getStatusCode()+','+r.getErrors()[0].getMessage()));
  }
  return String.join(out,'|');
 }
}
