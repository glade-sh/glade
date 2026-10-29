public class BusinessDefaultsProbe {
 public static List<String> observe(String prefix){
  Map<Id,RecordType> types=new Map<Id,RecordType>([SELECT Id,DeveloperName FROM RecordType WHERE SobjectType='Account']);
  Id explicitType;for(RecordType rt:types.values()){if(rt.DeveloperName=='Business_Default')explicitType=rt.Id;}
  List<String> rows=new List<String>{prefix+'_person_feature|'+Account.SObjectType.getDescribe().fields.getMap().containsKey('IsPersonAccount')+'|'};
  List<String> names=new List<String>{'business_omitted','business_explicit','empty'};
  List<Account> values=new List<Account>{new Account(Name='Probe'),new Account(Name='Explicit',RecordTypeId=explicitType),new Account()};
  for(Integer i=0;i<values.size();i++){
   Database.SaveResult r=Database.insert(values[i],false);String result;String message='';
   if(r.isSuccess()){Id accountId=values[i].Id;Account a=[SELECT RecordTypeId FROM Account WHERE Id=:accountId];result=types.get(a.RecordTypeId).DeveloperName+'|';}
   else{Database.Error e=r.getErrors()[0];result=String.valueOf(e.getStatusCode())+'|'+String.join(e.getFields(),',');message=e.getMessage();}
   rows.add(prefix+'_'+names[i]+'|'+result+'|message='+message);
  }
  return rows;
 }
}
