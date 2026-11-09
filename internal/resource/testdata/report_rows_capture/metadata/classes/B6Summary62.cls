@IsTest(SeeAllData=true) private class B6Summary62 {
 @IsTest static void capture() {
  insert new Account(Name='B6 Test Only');
  Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Summary' LIMIT 1];
  Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();
  Reports.ReportResults plain=Reports.ReportManager.runReport(r.Id,false);
  Reports.ReportResults detail=Reports.ReportManager.runReport(r.Id,true);
  System.assert(false,'name='+m.getName()+';format='+m.getReportFormat()+';columns='+m.getDetailColumns()+';groups='+m.getGroupingsDown().size()+';plainCount='+plain.getFactMap().get('T!T').getAggregates()[0].getValue()+';detailCount='+detail.getFactMap().get('T!T').getAggregates()[0].getValue()+';plainFacts='+plain.getFactMap().size()+';detailFacts='+detail.getFactMap().size()+';plainDetail='+plain.getHasDetailRows()+';detailDetail='+detail.getHasDetailRows());
 }
}
