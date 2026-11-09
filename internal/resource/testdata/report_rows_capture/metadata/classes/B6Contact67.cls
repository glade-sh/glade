@IsTest(SeeAllData=true) private class B6Contact67 {
 @IsTest static void capture() {
  Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Contacts' LIMIT 1];
  Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();
  Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,true);
  System.assert(false,'name='+m.getName()+';format='+m.getReportFormat()+';columns='+m.getDetailColumns()+';count='+x.getFactMap().get('T!T').getAggregates()[0].getValue()+';details='+x.getHasDetailRows());
 }
}
