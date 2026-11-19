@IsTest private class NullProbe62_00 { @IsTest static void m00(){
 PermissionSet ps=new PermissionSet(Name='SyntheticAssignment',Label='Synthetic Assignment');insert ps;
 Database.SaveResult missing=Database.insert(new PermissionSetAssignment(AssigneeId=UserInfo.getUserId()),false);
 Database.SaveResult control=Database.insert(new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(),PermissionSetId=ps.Id),false);
 System.assert(false,'CAPTURE|PermissionSetAssignment|'+JSON.serialize(new Map<String,Object>{'PermissionSetId'=>new Map<String,Object>{'nullSuccess'=>missing.isSuccess(),'nullErrors'=>missing.getErrors(),'controlSuccess'=>control.isSuccess(),'controlErrors'=>control.getErrors()}}));
} }