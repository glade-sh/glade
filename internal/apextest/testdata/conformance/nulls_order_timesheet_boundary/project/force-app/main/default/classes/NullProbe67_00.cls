@IsTest private class NullProbe67_00 { @IsTest static void m00(){
 ServiceResource sr=new ServiceResource(Name='Synthetic Resource',RelatedRecordId=UserInfo.getUserId(),IsActive=true);insert sr;
 Database.SaveResult missing=Database.insert(new TimeSheet(StartDate=Date.today(),EndDate=Date.today().addDays(7)),false);
 Database.SaveResult control=Database.insert(new TimeSheet(ServiceResourceId=sr.Id,StartDate=Date.today(),EndDate=Date.today().addDays(7)),false);
 System.assert(false,'CAPTURE|TimeSheet|'+JSON.serialize(new Map<String,Object>{'ServiceResourceId'=>new Map<String,Object>{'nullSuccess'=>missing.isSuccess(),'nullErrors'=>missing.getErrors(),'controlSuccess'=>control.isSuccess(),'controlErrors'=>control.getErrors()}}));
} }