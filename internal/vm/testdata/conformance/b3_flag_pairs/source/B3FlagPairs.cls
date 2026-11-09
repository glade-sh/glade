public class B3FlagPairs {
 public static String run() {
  String u=String.valueOf(Crypto.getRandomInteger());Contact c=new Contact(LastName='Synthetic'+u,Email='s'+u+'@example.invalid');insert c;List<String> out=new List<String>();
  for(String obj:new List<String>{'Lead','Case'}) for(String pair:new List<String>{'userother','userauto','otherauto'}) for(Integer i=0;i<4;i++) for(String route:new List<String>{'options','record'}) {
   SObject row;if(obj=='Lead')row=new Lead(LastName='Synthetic'+u+pair+i+route,Company='Synthetic');else row=new Case(Subject='Synthetic'+pair+i+route,Status='New',Origin='Phone',ContactId=c.Id);
   Database.DMLOptions o=new Database.DMLOptions();if(pair=='userother'||pair=='userauto')o.EmailHeader.triggerUserEmail=(i>=2);if(pair=='userother')o.EmailHeader.triggerOtherEmail=(Math.mod(i,2)==1);if(pair=='otherauto')o.EmailHeader.triggerOtherEmail=(i>=2);if(pair!='userother')o.EmailHeader.triggerAutoResponseEmail=(Math.mod(i,2)==1);
   try {Boolean success;if(route=='record'){row.setOptions(o);insert row;success=true;}else{Database.SaveResult r=Database.insert(row,o);success=r.isSuccess();}out.add(obj+'_'+pair+'_'+i+'_'+route+'='+success);}catch(Exception e){out.add(obj+'_'+pair+'_'+i+'_'+route+'=exception:'+e.getTypeName());}
  }return String.join(out,'|');
 }
}
