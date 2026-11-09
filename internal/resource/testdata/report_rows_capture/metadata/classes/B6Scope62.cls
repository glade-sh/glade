@IsTest(SeeAllData=true) private class B6Scope62 {
 @IsTest static void capture() {
  String out='';for(Report r:[SELECT Id,DeveloperName FROM Report WHERE DeveloperName IN ('B6_NoScope','B6_Accounts') ORDER BY DeveloperName]){
   Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,false);out+=r.DeveloperName+'='+x.getFactMap().get('T!T').getAggregates()[0].getValue()+';';
  }System.assert(false,out);
 }
}
