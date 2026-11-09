Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];
for(Integer shape=0;shape<2;shape++){
 try{Reports.ReportResults x=shape==0?Reports.ReportManager.runReport(r.Id,(Reports.ReportMetadata)null):Reports.ReportManager.runReport(r.Id,(Reports.ReportMetadata)null,true);System.debug('B6NULLMETA|'+shape+'|count='+x.getFactMap().get('T!T').getAggregates()[0].getValue()+'|details='+x.getHasDetailRows());}catch(Exception e){System.debug('B6NULLMETA|'+shape+'|'+e.getTypeName()+'|'+e.getMessage());}
}
