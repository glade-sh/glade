@IsTest private class FlowDateTest {
 private static void observe(String method,Date supplied,Integer days) {
  Map<String,Object> result=new Map<String,Object>();
  Contact row=new Contact(LastName='Grove',Birthdate=supplied);
  try {insert row;Contact stored=[SELECT Birthdate FROM Contact WHERE Id=:row.Id];result.put('matches',stored.Birthdate==Date.today().addDays(days));result.put('status','success');}
  catch(DmlException ex){result.put('status',String.valueOf(ex.getDmlStatusCode(0)));}
  System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(result));
 }
 @IsTest static void omittedDate(){observe('omittedDate',null,5);}
 @IsTest static void suppliedDate(){observe('suppliedDate',Date.today().addDays(8),8);}
 @IsTest static void invalidDate(){observe('invalidDate',Date.today().addDays(400),400);}
}
