@IsTest(SeeAllData=true) private with sharing class ReportProbe62 {
 static Map<String,Object> executeReport(Id id) {
  Map<String,Object> out=new Map<String,Object>();
  try {Reports.ReportResults x=Reports.ReportManager.runReport(id,true);Reports.ReportFactWithDetails f=(Reports.ReportFactWithDetails)x.getFactMap().get('T!T');out.put('count',f.getAggregates()[0].getValue());out.put('detailRows',f.getRows().size());}
  catch(Exception e){out.put('error',e.getTypeName()+': '+e.getMessage().left(180));}return out;
 }
 @IsTest static void probe(){Map<String,Object> obs=new Map<String,Object>();try{
  User actor=[SELECT Id FROM User WHERE Alias='probeb' LIMIT 1];Report r=[SELECT Id FROM Report WHERE DeveloperName='SyntheticQueue' LIMIT 1];obs.put('admin',executeReport(r.Id));
  System.runAs(actor){obs.put('reportQuery',[SELECT count() FROM Report WHERE Id=:r.Id]);try{obs.put('reportName',Reports.ReportManager.describeReport(r.Id).getReportMetadata().getName());}catch(Exception e){obs.put('describeError',e.getTypeName()+': '+e.getMessage().left(180));}obs.put('actor',executeReport(r.Id));}
 }catch(Exception e){obs.put('setupError',e.getTypeName()+': '+e.getMessage().left(180));}System.assert(false,'CAPTURE|Report|'+JSON.serialize(new Map<String,Object>{'sharing'=>obs}));}
}
