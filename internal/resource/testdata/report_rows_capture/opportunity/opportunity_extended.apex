Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Opportunities' LIMIT 1];System.debug('B6OPPEXT|'+JSON.serialize(Reports.ReportManager.describeReport(r.Id).getReportExtendedMetadata()));
