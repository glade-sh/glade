for (Report r:[SELECT Id,DeveloperName FROM Report WHERE DeveloperName IN ('B6_Accounts','B6_Contacts','B6_Summary') ORDER BY DeveloperName]) {
 Reports.ReportDescribeResult x=Reports.ReportManager.describeReport(r.Id);
 System.debug('B6EXT|'+r.DeveloperName+'|'+JSON.serialize(x.getReportExtendedMetadata()));
}
