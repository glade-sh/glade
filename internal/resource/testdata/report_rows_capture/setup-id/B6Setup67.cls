@IsTest private class B6Setup67 {
 @TestSetup static void seed(){insert new Account(Name='B6 Setup Only');}
 @IsTest static void capture(){
  Id reportId=Id.valueOf('<DEPLOYED_REPORT_ID>');Reports.ReportResults x=Reports.ReportManager.runReport(reportId,true);
  Reports.ReportFactWithDetails f=(Reports.ReportFactWithDetails)x.getFactMap().get('T!T');
  List<String> labels=new List<String>();for(Reports.ReportDetailRow row:f.getRows())labels.add(row.getDataCells()[0].getLabel());labels.sort();
  System.assert(false,'count='+f.getAggregates()[0].getValue()+';setupVisible='+labels.contains('B6 Setup Only'));
 }
}
