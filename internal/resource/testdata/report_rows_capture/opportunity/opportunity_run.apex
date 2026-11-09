Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Opportunities' LIMIT 1];
Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();
Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,true);
System.debug('B6OPP|metadata='+JSON.serialize(m)+'|count='+x.getFactMap().get('T!T').getAggregates()[0].getValue());
Map<String,Id> ids=new Map<String,Id>(); for(Opportunity o:[SELECT Id,Name FROM Opportunity]) ids.put(o.Name,o.Id);
Reports.ReportFactWithDetails fact=(Reports.ReportFactWithDetails)x.getFactMap().get('T!T');
for(Reports.ReportDetailRow row:fact.getRows()){List<Reports.ReportDataCell> c=row.getDataCells();System.debug('B6OPPCELL|name='+c[0].getLabel()+'|idMatch='+(c[0].getValue()==ids.get(c[0].getLabel()))+'|amountValue='+c[1].getValue()+'|amountLabel='+c[1].getLabel());}
