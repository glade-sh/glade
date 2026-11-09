PermissionSet ps=[SELECT Id FROM PermissionSet WHERE Name='R6_Bypass' LIMIT 1];
for(PermissionSetAssignment old : [SELECT Id FROM PermissionSetAssignment WHERE AssigneeId=:UserInfo.getUserId() AND PermissionSetId=:ps.Id]) delete old;
for(String mode : new List<String>{'future','past','revoked'}) {
 PermissionSetAssignment a=new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(),PermissionSetId=ps.Id);
 if(mode=='future') a.ExpirationDate=Datetime.now().addDays(2);
 if(mode=='past') a.ExpirationDate=Datetime.now().addDays(-2);
 insert a;
 if(mode=='revoked') { a.IsRevoked=true; update a; }
 List<PermissionSetAssignment> rows=[SELECT Id,IsActive,IsRevoked,ExpirationDate FROM PermissionSetAssignment WHERE Id=:a.Id ALL ROWS];
 if(rows.isEmpty()) System.debug('R6ACTIVE|'+mode+'|NOT_VISIBLE');
 else System.debug('R6ACTIVE|'+mode+'|'+String.valueOf(rows[0].IsActive)+'|'+String.valueOf(rows[0].IsRevoked));
 delete a;
}
