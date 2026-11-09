@IsTest(SeeAllData=true) private class B6Overload67 {
 @IsTest static void capture() {
  Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];
  Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();
  List<Reports.ReportResults> results=new List<Reports.ReportResults>{Reports.ReportManager.runReport(r.Id),Reports.ReportManager.runReport(r.Id,false),Reports.ReportManager.runReport(r.Id,true),Reports.ReportManager.runReport(r.Id,m),Reports.ReportManager.runReport(r.Id,m,false),Reports.ReportManager.runReport(r.Id,m,true)};
  String message='';
  for(Reports.ReportResults x:results) message += '['+x.getFactMap().get('T!T').getAggregates()[0].getValue()+','+x.getHasDetailRows()+']';
  System.assert(false,message);
 }
}
