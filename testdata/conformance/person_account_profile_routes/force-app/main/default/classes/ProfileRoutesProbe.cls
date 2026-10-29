public class ProfileRoutesProbe {
 public static List<String> observe(String prefix,Integer groupNumber) {
  Map<Id,RecordType> types=new Map<Id,RecordType>([SELECT Id,DeveloperName FROM RecordType WHERE SobjectType='Account']);
  Id patient;Id provider;Id business;
  for(RecordType rt:types.values()){if(rt.DeveloperName=='Patient'){patient=rt.Id;}if(rt.DeveloperName=='Provider'){provider=rt.Id;}if(rt.DeveloperName=='Business_Default'){business=rt.Id;}}
  List<String> routes=new List<String>{'statement_insert','statement_upsert','scalar','list','scalar_false','list_false','scalar_options','list_options','scalar_system','list_system','scalar_user','list_user','scalar_false_system','list_false_system','scalar_false_user','list_false_user'};
  List<String> names=new List<String>{'person_omitted','business_omitted','person_explicit','person_missing_last','empty','person_null','business_explicit'};
  List<String> rows=new List<String>();
  for(Integer route=groupNumber*4;route<groupNumber*4+4;route++) {
   List<Account> accounts=new List<Account>{new Account(LastName='Probe'),new Account(Name='Probe'),new Account(LastName='Explicit',RecordTypeId=patient),new Account(FirstName='Missing',RecordTypeId=provider),new Account(),new Account(LastName='Null',RecordTypeId=null),new Account(Name='Explicit',RecordTypeId=business)};
   for(Integer i=0;i<accounts.size();i++) {
    Account a=accounts[i];Database.SaveResult saved;
    Database.DMLOptions options=new Database.DMLOptions();options.optAllOrNone=false;
    String value;String message='';
    try {
     if(route==0){insert a;}
     else if(route==1){upsert a;}
     else if(route==2){saved=Database.insert(a);}
     else if(route==3){saved=Database.insert(new List<Account>{a})[0];}
     else if(route==4){saved=Database.insert(a,false);}
     else if(route==5){saved=Database.insert(new List<Account>{a},false)[0];}
     else if(route==6){saved=Database.insert(a,options);}
     else if(route==7){saved=Database.insert(new List<Account>{a},options)[0];}
     else if(route==8){saved=Database.insert(a,AccessLevel.SYSTEM_MODE);}
     else if(route==9){saved=Database.insert(new List<Account>{a},AccessLevel.SYSTEM_MODE)[0];}
     else if(route==10){saved=Database.insert(a,AccessLevel.USER_MODE);}
     else if(route==11){saved=Database.insert(new List<Account>{a},AccessLevel.USER_MODE)[0];}
     else if(route==12){saved=Database.insert(a,false,AccessLevel.SYSTEM_MODE);}
     else if(route==13){saved=Database.insert(new List<Account>{a},false,AccessLevel.SYSTEM_MODE)[0];}
     else if(route==14){saved=Database.insert(a,false,AccessLevel.USER_MODE);}
     else if(route==15){saved=Database.insert(new List<Account>{a},false,AccessLevel.USER_MODE)[0];}
     if(saved!=null && !saved.isSuccess()) {
      Database.Error e=saved.getErrors()[0];value=String.valueOf(e.getStatusCode())+'|'+String.join(e.getFields(),',');message=e.getMessage();
     } else {
      Id accountId=a.Id;Account actual=[SELECT RecordTypeId,IsPersonAccount FROM Account WHERE Id=:accountId];
      value=(actual.RecordTypeId==null?'null':types.get(actual.RecordTypeId).DeveloperName)+'|'+actual.IsPersonAccount;
     }
    } catch(DmlException e) {value=String.valueOf(e.getDmlStatusCode(0))+'|'+String.join(e.getDmlFieldNames(0),',');message=e.getDmlMessage(0);}
    rows.add(prefix+'_'+routes[route]+'_'+names[i]+'|'+value+'|message='+message);
   }
  }
  return rows;
 }
}
