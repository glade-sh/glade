public class B3RuleProbe {
 public static String run() {
  List<String> out=new List<String>();
  for(Integer i=0;i<3;i++) { Boolean enabled=i!=0;
   String unique=String.valueOf(Crypto.getRandomInteger()); Lead l=new Lead(LastName='Synthetic'+unique,Company=i==2?'Unmatched':'Synthetic',Email='synthetic'+unique+'@example.invalid');
   Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=enabled;
   Database.SaveResult r=Database.insert(l,o);
   Integer messages=[SELECT COUNT() FROM EmailMessage WHERE RelatedToId=:l.Id];
   List<Task> found=[SELECT Subject,Status,TaskSubtype FROM Task WHERE WhoId=:l.Id]; Integer tasks=found.size(); String detail=found.isEmpty()?'none':String.valueOf(found[0].Subject)+'~'+String.valueOf(found[0].Status)+'~'+String.valueOf(found[0].TaskSubtype);
   out.add((i==2?'lead_unmatched_true':'lead_auto_'+enabled)+'='+r.isSuccess()+','+messages+','+tasks+','+detail+(r.isSuccess()?'':','+r.getErrors()[0].getStatusCode()+','+r.getErrors()[0].getMessage()));
  }
  return String.join(out,'|');
 }
}
