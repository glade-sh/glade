@IsTest(SeeAllData=true) private class B6Opportunity67 { @IsTest static void capture() {
 Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Opportunities' LIMIT 1];
 Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,true);
 List<String> cells=new List<String>();for(Reports.ReportDetailRow row:((Reports.ReportFactWithDetails)x.getFactMap().get('T!T')).getRows()){List<Reports.ReportDataCell> c=row.getDataCells();cells.add(c[0].getLabel()+':'+c[1].getLabel()+':'+String.valueOf(c[1].getValue()));}cells.sort();
 System.assert(false,'type='+m.getReportType().getType()+';label='+m.getReportType().getLabel()+';columns='+m.getDetailColumns()+';count='+x.getFactMap().get('T!T').getAggregates()[0].getValue()+';cells='+cells);
} }