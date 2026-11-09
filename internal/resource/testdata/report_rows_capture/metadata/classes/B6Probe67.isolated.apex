@IsTest private class B6Probe67 {
 @IsTest static void capture() {
  String out = '';
  List<Report> rs = [SELECT Id, Name, DeveloperName, FolderName, Format FROM Report WHERE DeveloperName = 'B6_Accounts'];
  out += 'count=' + rs.size();
  if (!rs.isEmpty()) {
   Report r = rs[0];
   out += ';report=' + r.Name + ',' + r.DeveloperName + ',' + r.FolderName + ',' + r.Format;
   try { Reports.ReportMetadata m = Reports.ReportManager.describeReport(r.Id).getReportMetadata(); out += ';describe=' + m.getName() + ',' + m.getReportFormat() + ',' + m.getDetailColumns(); } catch (Exception e) { out += ';describe-error=' + e.getTypeName() + ',' + e.getMessage(); }
   try { Reports.ReportResults x = Reports.ReportManager.runReport(r.Id, true); out += ';run=' + x.getReportMetadata().getName() + ',' + x.getFactMap().size(); } catch (Exception e) { out += ';run-error=' + e.getTypeName() + ',' + e.getMessage(); }
  }
  System.assert(false, out);
 }
}
