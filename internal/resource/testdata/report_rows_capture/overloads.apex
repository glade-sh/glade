Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];
Reports.ReportMetadata metadata=Reports.ReportManager.describeReport(r.Id).getReportMetadata();
for (Integer shape=0;shape<6;shape++) {
 try {
  Reports.ReportResults result;
  if (shape==0) result=Reports.ReportManager.runReport(r.Id);
  if (shape==1) result=Reports.ReportManager.runReport(r.Id,false);
  if (shape==2) result=Reports.ReportManager.runReport(r.Id,true);
  if (shape==3) result=Reports.ReportManager.runReport(r.Id,metadata);
  if (shape==4) result=Reports.ReportManager.runReport(r.Id,metadata,false);
  if (shape==5) result=Reports.ReportManager.runReport(r.Id,metadata,true);
  System.debug('B6OVERLOAD|'+shape+'|count='+result.getFactMap().get('T!T').getAggregates()[0].getValue()+'|details='+result.getHasDetailRows());
 } catch(Exception e){System.debug('B6OVERLOAD|'+shape+'|'+e.getTypeName()+'|'+e.getMessage());}
}
