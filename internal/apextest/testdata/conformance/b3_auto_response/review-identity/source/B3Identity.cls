public class B3Identity {
 public static String run() {
  List<String> out=new List<String>();
  for(Integer i=0;i<2;i++) {
   Lead bad=new Lead(Id='00Q000000000001',LastName='Bad'+i,Company='Synthetic',Email='bad'+i+'@example.invalid');
   Lead good=new Lead(LastName='Good'+i,Company='Synthetic',Email='good'+i+'@example.invalid');
   Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=true;o.optAllOrNone=false;
   try {Database.SaveResult[] r=Database.insert(i==0?new List<Lead>{bad,good}:new List<Lead>{good,bad},o);Integer b=i==0?0:1;Integer g=1-b;Id goodId=r[g].getId();out.add('order'+i+'='+r[b].isSuccess()+','+r[g].isSuccess()+','+r[b].getErrors()[0].getStatusCode()+','+[SELECT COUNT() FROM Task WHERE WhoId=:goodId]);}
   catch(Exception e){out.add('order'+i+'=exception:'+e.getTypeName());}
  }
  return String.join(out,'|');
 }
}
