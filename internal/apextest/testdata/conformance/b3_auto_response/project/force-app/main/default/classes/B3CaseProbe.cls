public class B3CaseProbe {
 public static String run() {
  List<String> out=new List<String>();
  for(Integer i=0;i<3;i++) {
   Boolean enabled=i!=0; String unique=String.valueOf(Crypto.getRandomInteger());
   Case c=new Case(Subject=i==2?'Unmatched':'Synthetic',Status='New',Origin='Web',SuppliedEmail='synthetic'+unique+'@example.invalid');
   Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=enabled;
   Database.SaveResult r=Database.insert(c,o);
   List<Task> found=[SELECT Subject,Status,TaskSubtype FROM Task WHERE WhatId=:c.Id];
   String detail=found.isEmpty()?'none':String.valueOf(found[0].Subject)+'~'+String.valueOf(found[0].Status)+'~'+String.valueOf(found[0].TaskSubtype);
   out.add((i==2?'case_unmatched_true':'case_auto_'+enabled)+'='+r.isSuccess()+','+found.size()+','+detail+(r.isSuccess()?'':','+r.getErrors()[0].getStatusCode()));
  }
  return String.join(out,'|');
 }
}
