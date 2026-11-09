Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];
Reports.ReportTypeMetadata m=Reports.ReportManager.describeReport(r.Id).getReportTypeMetadata();
System.debug('B6TYPE|'+JSON.serialize(m));
