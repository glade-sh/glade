@IsTest(SeeAllData=true) private class B6Tx62 {
 @IsTest static void capture() {
  Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];
  Account first=[SELECT Id,Name FROM Account WHERE Name='B6 First' LIMIT 1];
  Account second=[SELECT Id,Name FROM Account WHERE Name='B6 Second' LIMIT 1];
  first.Name='B6 Modified'; update first; delete second;
  Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,true);
  Reports.ReportFactWithDetails fact=(Reports.ReportFactWithDetails)x.getFactMap().get('T!T');
  List<String> labels=new List<String>(); for(Reports.ReportDetailRow row:fact.getRows()) labels.add(row.getDataCells()[0].getLabel()); labels.sort();
  System.assert(false,'count='+fact.getAggregates()[0].getValue()+';labels='+labels);
 }
}
