@IsTest private class B6DirectIsolated62 {
 @IsTest static void capture() {
  Id reportId=Id.valueOf('<DEPLOYED_REPORT_ID>'); String out='';
  try { Reports.ReportMetadata m=Reports.ReportManager.describeReport(reportId).getReportMetadata(); out+='describe='+m.getName(); } catch(Exception e) { out+='describe-error='+e.getTypeName()+','+e.getMessage(); }
  try { Reports.ReportResults x=Reports.ReportManager.runReport(reportId,false); out+=';run='+x.getFactMap().get('T!T').getAggregates()[0].getValue(); } catch(Exception e) { out+=';run-error='+e.getTypeName()+','+e.getMessage(); }
  System.assert(false,out);
 }
}
