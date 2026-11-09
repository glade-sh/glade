Report r = [SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];
Reports.ReportDescribeResult d = Reports.ReportManager.describeReport(r.Id);
Reports.ReportMetadata m = d.getReportMetadata();
System.debug('B6DETAIL|metadata|' + JSON.serialize(m));
System.debug('B6DETAIL|extended-columns|' + d.getReportExtendedMetadata().getDetailColumnInfo().keySet());
for (Boolean details : new List<Boolean>{false,true}) {
 try {
  Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,details);
  System.debug('B6DETAIL|results-' + details + '|' + JSON.serialize(x));
 } catch(Exception e) {System.debug('B6DETAIL|error-' + details + '|' + e.getTypeName() + '|' + e.getMessage());}
}
