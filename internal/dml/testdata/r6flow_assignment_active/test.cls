@IsTest private class R6ActiveTest {
 static void observe(String mode) {
  PermissionSet ps=[SELECT Id FROM PermissionSet WHERE Name='R6_Bypass' LIMIT 1];
  PermissionSetAssignment a=new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(),PermissionSetId=ps.Id);
  if(mode=='future') a.ExpirationDate=Datetime.now().addDays(2);
  if(mode=='past') a.ExpirationDate=Datetime.now().addDays(-2);
  try { insert a; if(mode=='revoked'){a.IsRevoked=true;update a;} }
  catch(Exception e) { System.assert(false,'R6ACTIVE|'+mode+'|SETUP'); }
  a=[SELECT IsActive,IsRevoked FROM PermissionSetAssignment WHERE Id=:a.Id ALL ROWS];
  System.assert(false,'R6ACTIVE|'+mode+'|'+String.valueOf(a.IsActive)+'|'+String.valueOf(a.IsRevoked));
 }
 @IsTest static void future(){observe('future');}
 @IsTest static void past(){observe('past');}
 @IsTest static void revoked(){observe('revoked');}
}
