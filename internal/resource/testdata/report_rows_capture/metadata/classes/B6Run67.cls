@IsTest(SeeAllData=true) private class B6Run67 {
 @IsTest static void capture() {
  insert new Account(Name='B6 Test Only');
  Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];
  Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();
  Reports.ReportResults plain=Reports.ReportManager.runReport(r.Id,false);
  Reports.ReportResults detail=Reports.ReportManager.runReport(r.Id,true);
  Reports.ReportFactWithDetails f=(Reports.ReportFactWithDetails)detail.getFactMap().get('T!T');
  List<String> labels=new List<String>();
  for (Reports.ReportDetailRow row:f.getRows()) labels.add(row.getDataCells()[0].getLabel());
  labels.sort();
  System.assert(false,'name='+m.getName()+';format='+m.getReportFormat()+';columns='+m.getDetailColumns()+';plainCount='+plain.getFactMap().get('T!T').getAggregates()[0].getValue()+';detailCount='+f.getAggregates()[0].getValue()+';labels='+labels+';plainDetail='+plain.getHasDetailRows()+';detailDetail='+detail.getHasDetailRows());
 }
}
