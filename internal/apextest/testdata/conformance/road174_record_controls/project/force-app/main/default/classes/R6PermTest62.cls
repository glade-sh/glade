@IsTest(SeeAllData=true) private class R6PermTest62 {
 @IsTest static void observe(){User u=[SELECT Id FROM User WHERE LastName='Proof' AND IsActive=true LIMIT 1];PermissionSet p=[SELECT Id FROM PermissionSet WHERE Name='Road174Grant'];PermissionSetAssignment pa=[SELECT Id FROM PermissionSetAssignment WHERE AssigneeId=:u.Id AND PermissionSetId=:p.Id LIMIT 1];List<Object> out=new List<Object>();
 System.runAs(u){out.add(R6PermProbe62.get());}
 delete pa;
 System.runAs(u){out.add(R6PermProbe62.get());}
 insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=p.Id);
 System.runAs(u){out.add(R6PermProbe62.get());}
 System.assert(false,'R6PERM|'+JSON.serialize(out));}}
