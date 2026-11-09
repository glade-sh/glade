@IsTest(SeeAllData=true) private with sharing class B6Locale67 { @IsTest static void capture() {
 User u=[SELECT Id FROM User WHERE Alias='b6usr' LIMIT 1];Id id=[SELECT Id FROM Report WHERE DeveloperName='B6_Opportunities' LIMIT 1].Id;String out='';
 System.runAs(u){Reports.ReportFactWithDetails f=(Reports.ReportFactWithDetails)Reports.ReportManager.runReport(id,true).getFactMap().get('T!T');for(Reports.ReportDetailRow row:f.getRows()){List<Reports.ReportDataCell> c=row.getDataCells();out+=c[0].getLabel()+':'+c[1].getLabel()+';';}}System.assert(false,out);
} }